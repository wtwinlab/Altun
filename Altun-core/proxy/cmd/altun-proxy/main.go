// altun-proxy is the single client endpoint of an Altun deployment with two
// consensus instances over the same membership size: Paxi Paxos (CFT) and
// chained HotStuff (BFT).  Clients send KV ops to POST /kv; the proxy routes
// them to the instance that runs the active segment of the virtual log.  A
// signed policy-window directive (POST /admin/switch) moves the virtual log to
// the other instance:
//
//	quorum check -> stop admission -> drain -> seal (transition entry agreed
//	by the old instance) -> fetch + verify snapshot -> import into the new
//	instance -> segstart (first entry of the new instance; 2f+1 BFT replicas
//	must match the sealed digest) -> route -> release
//
// If activation fails the old instance resumes as the next segment (abort).
// On start the proxy rebuilds the segment table from the instances' replicated
// tables and completes an interrupted switch (replay).
package main

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/wtwinlab/altunproxy/directive"
	"github.com/wtwinlab/altunsm"
)

// ---------------------------------------------------------------- gate

// Gate controls admission of client ops and tracks in-flight ops.
type Gate struct {
	mu       sync.Mutex
	cond     *sync.Cond
	closed   bool
	inflight int
}

func newGate() *Gate { g := &Gate{}; g.cond = sync.NewCond(&g.mu); return g }

func (g *Gate) Enter() {
	g.mu.Lock()
	for g.closed {
		g.cond.Wait()
	}
	g.inflight++
	g.mu.Unlock()
}

func (g *Gate) Exit() {
	g.mu.Lock()
	g.inflight--
	if g.inflight == 0 {
		g.cond.Broadcast()
	}
	g.mu.Unlock()
}

// Stop stops admission of new ops.
func (g *Gate) Stop() {
	g.mu.Lock()
	g.closed = true
	g.mu.Unlock()
}

// Drain waits until every admitted op has returned.
func (g *Gate) Drain() {
	g.mu.Lock()
	for g.inflight > 0 {
		g.cond.Wait()
	}
	g.mu.Unlock()
}

func (g *Gate) Open() {
	g.mu.Lock()
	g.closed = false
	g.cond.Broadcast()
	g.mu.Unlock()
}

// ---------------------------------------------------------------- journal

type Journal struct {
	ch   chan []byte
	done chan struct{}
}

// openJournal starts the proxy's local event log.  Records are written by a
// background goroutine in order, so a slow (NFS) write never sits on the switch
// path; the authoritative copy of every transition record is the replicated
// segment table inside the consensus instances, not this file.
func openJournal(p string) *Journal {
	f, err := os.OpenFile(p, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		log.Fatal(err)
	}
	j := &Journal{ch: make(chan []byte, 4096), done: make(chan struct{})}
	go func() {
		for b := range j.ch {
			_, _ = f.Write(b)
		}
		_ = f.Sync()
		close(j.done)
	}()
	return j
}

func (j *Journal) Append(v any) {
	b, _ := json.Marshal(v)
	j.ch <- append(b, '\n')
}

// Flush waits until every queued record is written (used before an injected crash).
func (j *Journal) Flush() {
	for len(j.ch) > 0 {
		time.Sleep(time.Millisecond)
	}
	time.Sleep(5 * time.Millisecond)
}

// ---------------------------------------------------------------- proxy

type route struct {
	b   Backend
	seg uint64
}

type Proxy struct {
	swmu       sync.Mutex // serialises switches and recovery
	backends   map[string]Backend
	cur        atomic.Pointer[route]
	lastWindow uint64
	gate       *Gate
	j          *Journal
	pub        ed25519.PublicKey
	nswitch    int
	pid        int

	crashAt    string
	crashOn    int
	hookAt     string
	hookOn     int
	hookCmd    string
	noSeal     bool
	noVerify   bool
	noQuorum   bool
	fetchFrom  int
	actWait    time.Duration
	actAttempt time.Duration
	pids       map[string]int
	opTimeout  time.Duration

	// checkpoint certification
	cert     bool                         // certification enabled
	certPubs map[string]ed25519.PublicKey // attestation keys of every member (NAME.pub)
	certWait time.Duration                // attestation collection deadline
	certHash string                       // hash of the certificate installed with the active segment
}

const ctlClient = 0xFFFFFFFF

func now() int64 { return time.Now().UnixNano() }

func (p *Proxy) other(name string) Backend {
	for n, b := range p.backends {
		if n != name {
			return b
		}
	}
	return nil
}

// handleKV serves one client op.
func (p *Proxy) handleKV(w http.ResponseWriter, r *http.Request) {
	var op altunsm.Op
	if err := json.NewDecoder(r.Body).Decode(&op); err != nil || (op.T != "get" && op.T != "put") || op.C == 0 || op.C >= ctlClient {
		http.Error(w, "bad op", http.StatusBadRequest)
		return
	}
	catt, _ := strconv.ParseUint(r.Header.Get("Altun-Attempt"), 10, 64)
	base := op.S<<24 | (catt&0xfff)<<12
	var attempt uint64
	ctx, cancel := context.WithTimeout(r.Context(), p.opTimeout)
	defer cancel()
	for {
		p.gate.Enter()
		rt := p.cur.Load()
		res, raw, err := rt.b.Exec(ctx, op, uint32(op.C), base, &attempt)
		p.gate.Exit()
		if err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		if strings.HasPrefix(res.Err, "not-active") {
			// the contacted instance is not (or no longer) the active segment
			if p.cur.Load() == rt {
				rt.b.Rotate(rt.b.Leader())
				time.Sleep(10 * time.Millisecond)
			}
			if ctx.Err() != nil {
				http.Error(w, "not active", http.StatusServiceUnavailable)
				return
			}
			continue
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(raw)
		return
	}
}

// SwitchReport is returned by /admin/switch and journaled.
type SwitchReport struct {
	Kind     string           `json:"kind"` // switch
	Window   uint64           `json:"window"`
	From     string           `json:"from"`
	To       string           `json:"to"`
	OldSeg   uint64           `json:"old_seg"`
	NewSeg   uint64           `json:"new_seg"`
	Tail     uint64           `json:"tail"`
	Digest   string           `json:"digest"`
	OK       bool             `json:"ok"`
	Aborted  bool             `json:"aborted"`
	Reason   string           `json:"reason,omitempty"`
	Phases   map[string]int64 `json:"phases"` // unix ns
	Rejected []string         `json:"rejected,omitempty"`
	Imported int              `json:"imported"`
	Matched  int              `json:"matched"`
	Recovery bool             `json:"recovery,omitempty"`
	Unsafe   string           `json:"unsafe,omitempty"`
	Pid      int              `json:"pid"`
	// checkpoint certification
	FB           int      `json:"fb,omitempty"`
	CertQuorum   int      `json:"cert_quorum,omitempty"`
	CertMatched  int      `json:"cert_matched,omitempty"`
	CertRejected []string `json:"cert_rejected,omitempty"`
	Cert         string   `json:"cert,omitempty"`        // hash of the certificate installed by this switch
	ActMatched   int      `json:"act_matched,omitempty"` // CFT: matching start attestations
	ActRejected  []string `json:"act_rejected,omitempty"`
}

// hook runs a fault-injection command at a switch phase.
func (p *Proxy) hook(point string, old Backend) {
	if p.hookAt != point || p.nswitch != p.hookOn || p.hookCmd == "" {
		return
	}
	if p.hookCmd == "kill-old-leader" {
		// built-in: SIGKILL the old instance's contact replica (Paxos: the
		// leader the proxy talks to; HotStuff: replica LEADER) without forking
		// a shell, so the fault lands inside the drain/seal window.
		name := fmt.Sprintf("paxos%d", old.Leader()+1)
		if old.Name() == "hotstuff" {
			name = fmt.Sprintf("hs%d", old.Leader()+1)
		}
		pid := p.pids[name]
		t0 := now()
		err := syscall.Kill(pid, syscall.SIGKILL)
		p.j.Append(map[string]any{"kind": "hook", "point": point, "switch": p.nswitch, "t": t0, "killed": name, "pid": pid, "err": fmt.Sprint(err)})
		return
	}
	c := exec.Command("bash", "-c", p.hookCmd)
	c.Env = append(os.Environ(), fmt.Sprintf("LEADER=%d", old.Leader()+1), "OLD="+old.Name())
	out, err := c.CombinedOutput()
	p.j.Append(map[string]any{"kind": "hook", "point": point, "switch": p.nswitch, "t": now(), "out": string(out), "err": fmt.Sprint(err), "old": old.Name(), "leader": old.Leader() + 1})
}

func (p *Proxy) crash(point string) {
	if p.crashAt == point && p.nswitch == p.crashOn {
		p.j.Append(map[string]any{"kind": "crash", "point": point, "t": now(), "pid": p.pid})
		log.Printf("CRASH HOOK at %s (switch %d)", point, p.nswitch)
		p.j.Flush()
		os.Exit(3)
	}
}

func (p *Proxy) handleSwitch(w http.ResponseWriter, r *http.Request) {
	tRecv := now()
	var d directive.Directive
	if err := json.NewDecoder(r.Body).Decode(&d); err != nil {
		http.Error(w, "bad directive", http.StatusBadRequest)
		return
	}
	if err := d.Verify(p.pub); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	rep := p.doSwitch(d, tRecv)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(rep)
}

// quorumCheck: the quorum-preserving membership condition of the target
// instance, checked before anything is sealed.
func (p *Proxy) quorumCheck(t Backend, d directive.Directive) string {
	if d.N != t.N() {
		return fmt.Sprintf("directive n=%d differs from instance membership %d", d.N, t.N())
	}
	if t.Family() == "BFT" {
		if d.N < 3*d.F+1 {
			return fmt.Sprintf("BFT needs n >= 3f+1 (n=%d f=%d)", d.N, d.F)
		}
		if d.F > t.F() {
			return fmt.Sprintf("instance configured for f=%d < directive f=%d", t.F(), d.F)
		}
	} else if d.N < 2*d.F+1 {
		return fmt.Sprintf("CFT needs n >= 2f+1 (n=%d f=%d)", d.N, d.F)
	}
	if p.cert {
		fb := d.CertBound()
		if fb < 0 || (fb > 0 && d.N < 3*fb+1) {
			return fmt.Sprintf("certification needs n >= 3f_B+1 (n=%d f_B=%d)", d.N, fb)
		}
	}
	need := t.N() - d.F // BFT: 2f+1 when n=3f+1; CFT: majority when n=2f+1
	if t.Family() == "CFT" {
		need = t.N()/2 + 1
	}
	live := 0
	for _, u := range t.Admin() {
		var st altunsm.Status
		if _, err := getJSON(u+"/altun/status", &st, 500*time.Millisecond); err == nil {
			live++
		}
	}
	if live < need {
		return fmt.Sprintf("only %d of %d members reachable, need %d", live, t.N(), need)
	}
	return ""
}

func (p *Proxy) doSwitch(d directive.Directive, tRecv int64) SwitchReport {
	p.swmu.Lock()
	defer p.swmu.Unlock()
	p.nswitch++
	rt := p.cur.Load()
	rep := SwitchReport{Kind: "switch", Window: d.Window, From: rt.b.Name(), To: d.To, OldSeg: rt.seg,
		Phases: map[string]int64{"recv": tRecv}, Pid: p.pid}
	fail := func(reason string) SwitchReport {
		rep.Reason = reason
		rep.Phases["end"] = now()
		p.j.Append(rep)
		return rep
	}
	if d.Window <= p.lastWindow {
		return fail(fmt.Sprintf("stale window %d <= %d", d.Window, p.lastWindow))
	}
	t, ok := p.backends[d.To]
	if !ok || t == rt.b {
		return fail("target is not the other instance")
	}
	if !p.noQuorum {
		if why := p.quorumCheck(t, d); why != "" {
			rep.Aborted = true
			return fail("quorum check: " + why)
		}
	}
	rep.Phases["checked"] = now()
	if p.noSeal {
		return p.unsafeNoSealSwitch(d, rt, t, rep)
	}
	k := rt.seg
	fb := d.CertBound()
	rep.FB = fb
	rec := &altunsm.Record{Window: d.Window, From: rt.b.Name(), To: t.Name(), NewSeg: k + 1, F: d.F, N: d.N, FB: fb, Ts: now()}

	// 1. stop admission and drain the old segment
	p.gate.Stop()
	rep.Phases["stopped"] = now()
	p.hook("stopped", rt.b)
	p.gate.Drain()
	rep.Phases["drained"] = now()
	p.hook("drained", rt.b)
	defer func() {
		p.gate.Open()
		t := now()
		rep.Phases["released"] = t
		p.j.Append(map[string]any{"kind": "released", "window": d.Window, "t": t, "pid": p.pid})
	}()

	// 2. seal: the transition entry is ordered by the OLD instance
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	var at uint64
	res, _, err := rt.b.Exec(ctx, altunsm.Op{T: "seal", Seg: k, Rec: rec}, ctlClient, uint64(now()), &at)
	cancel()
	if err != nil || !res.OK {
		rep.Reason = fmt.Sprintf("seal failed: %v %s", err, res.Err)
		rep.Phases["end"] = now()
		p.j.Append(rep)
		return rep // old segment not sealed: stays active
	}
	rep.Tail, rep.Digest = res.Tail, res.Digest
	rep.Phases["sealed"] = now()
	p.lastWindow = d.Window
	p.j.Append(map[string]any{"kind": "sealed", "seg": k, "window": d.Window, "tail": res.Tail, "digest": res.Digest, "to": t.Name(), "t": now(), "pid": p.pid})
	p.hook("sealed", rt.b)
	p.crash("after-seal")

	// 2b. certification: the sealed checkpoint must be attested by q_c = n - f_B members
	var cert *altunsm.Certificate
	tail, digest := res.Tail, res.Digest
	if p.cert {
		var oc certOutcome
		var why string
		cert, oc, why = p.certify(rt.b, k, fb, d.N, &rep)
		rep.Phases["certified"] = now()
		if cert == nil {
			// resume on the sealed state that at least f_B+1 members (so at
			// least one correct one) attested; otherwise on the seal reply
			tail, digest = oc.resumeDigest(fb, tail, digest)
			rep.Tail, rep.Digest = tail, digest
			return p.abort(rt.b, k, digest, d.Window, rep, why)
		}
		tail, digest = cert.Tail, cert.Digest
		rep.Tail, rep.Digest = tail, digest
	}

	return p.completeSwitch(rt.b, t, k, tail, digest, d.Window, d.F, d.N, cert, rep)
}

// completeSwitch runs transfer + activation for a sealed segment k (also used by recovery).
func (p *Proxy) completeSwitch(old, t Backend, k, tail uint64, digest string, window uint64, f, n int, cert *altunsm.Certificate, rep SwitchReport) SwitchReport {
	// 3. fetch the sealed state and verify it against the agreed digest
	snap, why := p.fetchSnapshot(old, k, digest)
	if snap == nil {
		return p.abort(old, k, digest, window, rep, "snapshot: "+why)
	}
	rep.Phases["fetched"] = now()

	// 4. import into the new instance (out of band, staged)
	newSeg := k + 1
	var wg sync.WaitGroup
	var mu sync.Mutex
	acks := 0
	for i, u := range t.Admin() {
		wg.Add(1)
		go func(i int, u string) {
			defer wg.Done()
			if _, err := postJSON(u+"/altun/import", altunsm.ImportReq{NewSeg: newSeg, Snap: snap}, 5*time.Second); err == nil {
				mu.Lock()
				acks++
				mu.Unlock()
			}
		}(i, u)
	}
	wg.Wait()
	rep.Imported = acks
	rep.Phases["imported"] = now()
	p.crash("after-import")
	need := t.N()/2 + 1
	if t.Family() == "BFT" {
		need = t.N() - t.F()
	}
	if acks < need {
		return p.abort(old, k, digest, window, rep, fmt.Sprintf("only %d imports acknowledged, need %d", acks, need))
	}

	// 5. activation: segstart is the first entry of the new instance
	op := altunsm.Op{T: "segstart", Seg: newSeg, Digest: digest, Cert: cert,
		Rec: &altunsm.Record{Kind: "activate", Window: window, From: old.Name(), To: t.Name(), F: f, N: n, Ts: now()}}
	certHash := p.certHash
	if cert != nil {
		op.Rec.FB = cert.FB
		certHash = cert.Hash()
	}
	matched := 0
	if t.Family() == "BFT" {
		deadline := time.Now().Add(p.actWait)
		for attempt := uint64(0); matched < need && time.Now().Before(deadline); attempt++ {
			matched = 0
			rep.Rejected = nil
			per := time.Until(deadline)
			if per > p.actAttempt {
				per = p.actAttempt
			}
			replied := 0
			for _, rp := range t.Gather(op, ctlClient, uint64(now()), per) {
				if rp.Err == nil && rp.Res.Err != "forked" {
					replied++
				}
				if rp.Err == nil && rp.Res.OK && rp.Res.Digest == digest && rp.Res.Seg == newSeg {
					matched++
				} else {
					why := "no reply"
					if rp.Err == nil {
						why = fmt.Sprintf("ok=%v err=%s digest=%.12s", rp.Res.OK, rp.Res.Err, rp.Res.Digest)
					}
					rep.Rejected = append(rep.Rejected, fmt.Sprintf("r%d: %s", rp.Member+1, why))
				}
			}
			if replied == t.N() {
				break // every member answered: the outcome is final, do not wait for the deadline
			}
		}
	} else {
		deadline := time.Now().Add(p.actWait)
		ctx, cancel := context.WithTimeout(context.Background(), p.actWait)
		var at uint64
		res, _, err := t.Exec(ctx, op, ctlClient, uint64(now()), &at)
		cancel()
		if err == nil && res.OK && res.Digest == digest && res.Seg == newSeg {
			matched = 1
			need = 1
		} else {
			rep.Rejected = append(rep.Rejected, fmt.Sprintf("leader: err=%v %s", err, res.Err))
		}
		if p.cert && cert != nil {
			// certified activation: the same quorum as the certificate, q_c signed
			// start attestations <k+1, tail, digest, hash(cert)> from distinct members
			need = cert.Quorum()
			oc := p.collect(t, "start", newSeg, certHash, need, deadline)
			matched = 0
			for _, a := range oc.att {
				if a.Digest == digest && a.Tail == tail {
					matched++
				}
			}
			rep.ActMatched, rep.ActRejected = matched, oc.rejected
			for _, r := range oc.rejected {
				rep.Rejected = append(rep.Rejected, r)
			}
		}
	}
	rep.Matched = matched
	if matched < need {
		return p.abort(old, k, digest, window, rep, fmt.Sprintf("activation: %d matching replies, need %d", matched, need))
	}
	rep.Phases["activated"] = now()
	rep.NewSeg = newSeg
	rep.OK = true
	if cert != nil {
		p.certHash = certHash
		rep.Cert = certHash
	}
	p.cur.Store(&route{b: t, seg: newSeg})
	rep.Phases["routed"] = now()
	p.j.Append(rep)
	return rep
}
