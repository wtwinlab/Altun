package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/wtwinlab/altunproxy/directive"
	"github.com/wtwinlab/altunsm"
)

// abort keeps the virtual log on the old instance: it resumes on its unchanged
// sealed state as segment k+2 (segment k+1 stays registered but empty).
func (p *Proxy) abort(old Backend, k uint64, digest string, window uint64, rep SwitchReport, why string) SwitchReport {
	rep.Aborted = true
	rep.Reason = why
	rep.Phases["abort"] = now()
	op := altunsm.Op{T: "segstart", Seg: k + 2, Digest: digest,
		Rec: &altunsm.Record{Kind: "abort", Window: window, From: old.Name(), To: old.Name(), Ts: now()}}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	var at uint64
	res, _, err := old.Exec(ctx, op, ctlClient, uint64(now()), &at)
	cancel()
	if err != nil || !res.OK {
		rep.Reason += fmt.Sprintf("; RESUME FAILED: %v %s", err, res.Err)
		log.Printf("abort: resume failed: %v %s", err, res.Err)
	} else {
		p.cur.Store(&route{b: old, seg: k + 2})
		rep.NewSeg = k + 2
	}
	rep.Phases["routed"] = now()
	p.j.Append(rep)
	return rep
}

// fetchSnapshot gets the state of sealed segment k from some member of old and
// verifies it against the digest agreed in the seal entry.
func (p *Proxy) fetchSnapshot(old Backend, k uint64, digest string) (*altunsm.Snapshot, string) {
	n := old.N()
	start := old.Leader()
	if p.fetchFrom >= 0 {
		start = p.fetchFrom
	} else if !p.noVerify {
		return p.fetchParallel(old, k, digest)
	}
	why := ""
	for i := 0; i < n; i++ {
		m := (start + i) % n
		var s altunsm.Snapshot
		if _, err := getJSON(fmt.Sprintf("%s/altun/snapshot?seg=%d", old.Admin()[m], k), &s, 5*time.Second); err != nil {
			why += fmt.Sprintf("r%d: %v; ", m+1, err)
			continue
		}
		if p.noVerify {
			return &s, ""
		}
		if got := s.Digest(); got != digest {
			why += fmt.Sprintf("r%d: digest %.12s != sealed %.12s (rejected); ", m+1, got, digest)
			p.j.Append(map[string]any{"kind": "snapshot-rejected", "member": m + 1, "seg": k, "t": now()})
			continue
		}
		return &s, why
	}
	return nil, why
}

// unsafeNoSealSwitch activates the new instance before sealing the old one,
// without stopping admission or draining.  The state is captured with a
// non-sealing peek; client ops keep flowing to the old instance until the
// route flips, and the old instance is sealed only afterwards.  It exists to
// demonstrate the consequence of omitting the seal (-unsafe-noseal).
func (p *Proxy) unsafeNoSealSwitch(d directive.Directive, rt *route, t Backend, rep SwitchReport) SwitchReport {
	rep.Unsafe = "noseal"
	k := rt.seg
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var at uint64
	res, _, err := rt.b.Exec(ctx, altunsm.Op{T: "peek"}, ctlClient, uint64(now()), &at)
	if err != nil || !res.OK {
		rep.Reason = "peek failed"
		p.j.Append(rep)
		return rep
	}
	rep.Tail, rep.Digest = res.Tail, res.Digest
	rep.Phases["sealed"] = now()
	p.lastWindow = d.Window
	snap, why := p.fetchSnapshot(rt.b, k, res.Digest)
	if snap == nil {
		rep.Reason = why
		p.j.Append(rep)
		return rep
	}
	rep.Phases["fetched"] = now()
	for _, u := range t.Admin() {
		if _, err := postJSON(u+"/altun/import", altunsm.ImportReq{NewSeg: k + 1, Snap: snap}, 5*time.Second); err == nil {
			rep.Imported++
		}
	}
	rep.Phases["imported"] = now()
	op := altunsm.Op{T: "segstart", Seg: k + 1, Digest: res.Digest, Rec: &altunsm.Record{Kind: "activate", Window: d.Window, From: rt.b.Name(), To: t.Name()}}
	var at2 uint64
	if r2, _, err := t.Exec(ctx, op, ctlClient, uint64(now()), &at2); err != nil || !r2.OK {
		rep.Reason = "activation failed"
	}
	rep.Phases["activated"] = now()
	p.cur.Store(&route{b: t, seg: k + 1})
	rep.Phases["routed"] = now()
	// late seal of the old instance
	var at3 uint64
	_, _, _ = rt.b.Exec(ctx, altunsm.Op{T: "seal", Seg: k, Rec: &altunsm.Record{Window: d.Window, From: rt.b.Name(), To: t.Name(), NewSeg: k + 1}}, ctlClient, uint64(now()), &at3)
	rep.Phases["released"] = now()
	rep.OK = true
	rep.NewSeg = k + 1
	p.j.Append(rep)
	return rep
}

// ---------------------------------------------------------------- recovery

type view struct {
	b     Backend
	st    altunsm.Status
	ok    bool
	count int
}

func stateRank(s string) int {
	switch s {
	case altunsm.Sealed:
		return 3
	case altunsm.Active:
		return 2
	case altunsm.Staged:
		return 1
	}
	return 0
}

// trustedView returns the instance's status as agreed by its fault model:
// CFT: the most advanced reachable replica; BFT: a status reported identically
// by at least f+1 replicas (the most advanced such).
func trustedView(b Backend) view {
	var sts []altunsm.Status
	for _, u := range b.Admin() {
		var st altunsm.Status
		if _, err := getJSON(u+"/altun/status", &st, time.Second); err == nil {
			sts = append(sts, st)
		}
	}
	v := view{b: b}
	key := func(s altunsm.Status) string {
		t, _ := json.Marshal(s.Table)
		h := sha256.Sum256(t)
		return fmt.Sprintf("%s|%d|%s|%s", s.State, s.Seg, s.SealDigest, hex.EncodeToString(h[:8]))
	}
	cnt := map[string]int{}
	for _, s := range sts {
		cnt[key(s)]++
	}
	need := 1
	if b.Family() == "BFT" {
		need = b.F() + 1
	}
	for _, s := range sts {
		if cnt[key(s)] < need {
			continue
		}
		if !v.ok || s.Seg > v.st.Seg || (s.Seg == v.st.Seg && stateRank(s.State) > stateRank(v.st.State)) {
			v.st, v.ok, v.count = s, true, cnt[key(s)]
		}
	}
	return v
}

// recover rebuilds the segment table from the replicated tables and completes
// an interrupted switch.
func (p *Proxy) recover(initial string) {
	p.swmu.Lock()
	defer p.swmu.Unlock()
	t0 := now()
	var views []view
	for _, b := range p.backends {
		views = append(views, trustedView(b))
	}
	var best *view
	for i := range views {
		v := &views[i]
		if !v.ok || v.st.State == altunsm.Fresh {
			continue
		}
		if best == nil || v.st.Seg > best.st.Seg || (v.st.Seg == best.st.Seg && stateRank(v.st.State) > stateRank(best.st.State)) {
			best = v
		}
	}
	for _, v := range views {
		if v.ok {
			for _, r := range v.st.Table {
				if r.Window > p.lastWindow {
					p.lastWindow = r.Window
				}
			}
		}
	}
	if best == nil {
		// fresh deployment: bootstrap segment 1 on the initial instance
		b := p.backends[initial]
		empty := &altunsm.Snapshot{Seg: 0, Tail: 0, KV: map[string]string{}, Dedup: map[uint64]altunsm.DedupEntry{}}
		dg := empty.Digest()
		for _, u := range b.Admin() {
			if _, err := postJSON(u+"/altun/import", altunsm.ImportReq{NewSeg: 1, Snap: empty}, 5*time.Second); err != nil {
				log.Printf("bootstrap import %s: %v", u, err)
			}
		}
		op := altunsm.Op{T: "segstart", Seg: 1, Digest: dg, Rec: &altunsm.Record{Kind: "activate", To: b.Name(), Ts: now()}}
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		var at uint64
		res, _, err := b.Exec(ctx, op, ctlClient, uint64(now()), &at)
		if err != nil || !res.OK {
			log.Fatalf("bootstrap failed: %v %+v", err, res)
		}
		p.cur.Store(&route{b: b, seg: 1})
		p.j.Append(map[string]any{"kind": "bootstrap", "to": b.Name(), "seg": 1, "digest": dg, "t0": t0, "t": now(), "pid": p.pid})
		log.Printf("bootstrapped segment 1 on %s", b.Name())
		return
	}
	table := best.st.Table
	p.j.Append(map[string]any{"kind": "replay", "from": best.b.Name(), "state": best.st.State, "seg": best.st.Seg, "table": table, "t": now(), "pid": p.pid})
	switch best.st.State {
	case altunsm.Active:
		p.cur.Store(&route{b: best.b, seg: best.st.Seg})
		p.certHash = installedCert(table)
		p.j.Append(map[string]any{"kind": "recovered", "active": best.b.Name(), "seg": best.st.Seg, "t0": t0, "t": now(), "pid": p.pid})
		log.Printf("recovered: %s active in segment %d (%d records)", best.b.Name(), best.st.Seg, len(table))
	case altunsm.Sealed:
		// interrupted between seal (registration of k+1) and activation: replay
		last := table[len(table)-1]
		t := p.backends[last.To]
		k := best.st.Seg
		rep := SwitchReport{Kind: "switch", Window: last.Window, From: best.b.Name(), To: last.To, OldSeg: k,
			Tail: last.Tail, Digest: best.st.SealDigest, Recovery: true, Phases: map[string]int64{"recv": t0, "sealed": t0}, Pid: p.pid}
		p.cur.Store(&route{b: best.b, seg: k})
		p.certHash = installedCert(table)
		var cert *altunsm.Certificate
		tail, digest := last.Tail, best.st.SealDigest
		if p.cert {
			fb := last.FB
			var why string
			cert, _, why = p.certify(best.b, k, fb, last.N, &rep)
			rep.Phases["certified"] = now()
			if cert == nil {
				rep = p.abort(best.b, k, digest, last.Window, rep, why)
				log.Printf("recovered: certification of segment %d failed, aborted: %s", k, why)
				return
			}
			tail, digest = cert.Tail, cert.Digest
		}
		rep = p.completeSwitch(best.b, t, k, tail, digest, last.Window, last.F, last.N, cert, rep)
		log.Printf("recovered by replaying the transition of segment %d -> %s: ok=%v aborted=%v %s", k, last.To, rep.OK, rep.Aborted, rep.Reason)
	default:
		log.Fatalf("recovery: unexpected state %+v", best.st)
	}
}

// fetchParallel asks every member for the sealed state and takes the first
// copy whose digest equals the agreed seal digest.
func (p *Proxy) fetchParallel(old Backend, k uint64, digest string) (*altunsm.Snapshot, string) {
	type got struct {
		m   int
		s   *altunsm.Snapshot
		err error
	}
	ch := make(chan got, old.N())
	t0 := now()
	var trace []string
	defer func() {
		p.j.Append(map[string]any{"kind": "fetch", "seg": k, "t0": t0, "t": now(), "trace": trace})
	}()
	for m := 0; m < old.N(); m++ {
		go func(m int) {
			// a lagging replica may not have executed the seal yet: retry briefly
			var err error
			for t := 0; t < 50; t++ {
				var s altunsm.Snapshot
				if _, err = getJSON(fmt.Sprintf("%s/altun/snapshot?seg=%d", old.Admin()[m], k), &s, 5*time.Second); err == nil {
					ch <- got{m, &s, nil}
					return
				}
				if !strings.Contains(err.Error(), "409") {
					break
				}
				time.Sleep(20 * time.Millisecond)
			}
			ch <- got{m, nil, err}
		}(m)
	}
	why := ""
	for i := 0; i < old.N(); i++ {
		g := <-ch
		trace = append(trace, fmt.Sprintf("r%d +%.1fms err=%v", g.m+1, float64(now()-t0)/1e6, g.err))
		if g.err != nil {
			why += fmt.Sprintf("r%d: %v; ", g.m+1, g.err)
			continue
		}
		if d := g.s.Digest(); d != digest {
			why += fmt.Sprintf("r%d: digest %.12s != sealed %.12s (rejected); ", g.m+1, d, digest)
			p.j.Append(map[string]any{"kind": "snapshot-rejected", "member": g.m + 1, "seg": k, "t": now()})
			continue
		}
		return g.s, why
	}
	return nil, why
}
