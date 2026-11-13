package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/wtwinlab/altunproxy/directive"
)

func main() {
	listen := flag.String("listen", "127.0.0.1:37400", "client endpoint")
	paxos := flag.String("paxos", "", "comma-separated Paxi http addresses (ids 1.1..1.n in order)")
	hs := flag.String("hotstuff", "", "comma-separated HotStuff KV front ends (replicas 1..n in order)")
	f := flag.Int("f", 1, "HotStuff fault threshold")
	pub := flag.String("pub", "", "directive public key file")
	journal := flag.String("journal", "segments.jsonl", "proxy journal")
	initial := flag.String("initial", "paxos", "instance of segment 1 on a fresh deployment")
	crashAt := flag.String("crash-at", "", "fault injection: exit at after-seal | after-import")
	crashOn := flag.Int("crash-on", 1, "fault injection: ... during the n-th switch of this process")
	noSeal := flag.Bool("unsafe-noseal", false, "disables the seal: activate before sealing, no drain (demonstrates the consequence; unsafe)")
	noVerify := flag.Bool("unsafe-noverify", false, "disables verification of the fetched snapshot (unsafe)")
	noQuorum := flag.Bool("unsafe-noquorum", false, "disables the quorum-preserving admission check (unsafe)")
	hookAt := flag.String("hook-at", "", "fault injection: run -hook-cmd at stopped | drained | sealed")
	hookOn := flag.Int("hook-on", 1, "fault injection: ... during the n-th switch of this process")
	hookCmd := flag.String("hook-cmd", "", "fault injection: kill-old-leader, or a bash command (env LEADER=1-based contact member of the old instance, OLD=name)")
	fetchFrom := flag.Int("fetch-from", -1, "member index (0-based) to fetch snapshots from first (-1: parallel fetch from every member)")
	attCFT := flag.Duration("attempt-cft", time.Second, "per-attempt timeout towards Paxos")
	attBFT := flag.Duration("attempt-bft", 400*time.Millisecond, "per-attempt timeout towards HotStuff")
	actWait := flag.Duration("act-wait", 10*time.Second, "activation deadline")
	actAttempt := flag.Duration("act-attempt", time.Second, "per-attempt timeout of the BFT activation entry")
	pidsFile := flag.String("pids", "", "name/pid file of the local cluster (for -hook-cmd kill-old-leader)")
	opTimeout := flag.Duration("op-timeout", 60*time.Second, "client op deadline inside the proxy")
	cert := flag.Bool("cert", false, "certify every sealed checkpoint with q_c = n - f_B signed attestations before transfer; the same quorum acknowledges a CFT activation")
	certKeys := flag.String("cert-keys", "", "directory with the members' attestation public keys (NAME.pub)")
	certWait := flag.Duration("cert-wait", 3*time.Second, "attestation collection deadline")
	flag.Parse()

	pk, err := directive.LoadPub(*pub)
	if err != nil {
		log.Fatal(err)
	}
	p := &Proxy{backends: map[string]Backend{}, gate: newGate(), j: openJournal(*journal), pub: pk, pid: os.Getpid(),
		crashAt: *crashAt, crashOn: *crashOn, hookAt: *hookAt, hookOn: *hookOn, hookCmd: *hookCmd, noSeal: *noSeal, noVerify: *noVerify, noQuorum: *noQuorum,
		fetchFrom: *fetchFrom, actWait: *actWait, actAttempt: *actAttempt, pids: map[string]int{}, opTimeout: *opTimeout,
		cert: *cert, certWait: *certWait}
	if *cert {
		p.certPubs = loadCertPubs(*certKeys)
	}
	if *pidsFile != "" {
		if b, err := os.ReadFile(*pidsFile); err == nil {
			for _, l := range strings.Split(string(b), "\n") {
				var n string
				var id int
				if c, _ := fmt.Sscan(l, &n, &id); c == 2 {
					p.pids[n] = id
				}
			}
		}
	}
	pb := &paxosBackend{attempt: *attCFT}
	for i, u := range strings.Split(*paxos, ",") {
		pb.urls = append(pb.urls, u)
		pb.ids = append(pb.ids, "1."+string(rune('1'+i)))
	}
	p.backends["paxos"] = pb
	p.backends["hotstuff"] = &hotstuffBackend{urls: strings.Split(*hs, ","), f: *f, attempt: *attBFT}
	p.j.Append(map[string]any{"kind": "start", "pid": p.pid, "t": now(), "args": os.Args})

	p.recover(*initial)

	mux := http.NewServeMux()
	mux.HandleFunc("/kv", p.handleKV)
	mux.HandleFunc("/admin/switch", p.handleSwitch)
	mux.HandleFunc("/admin/status", func(w http.ResponseWriter, r *http.Request) {
		rt := p.cur.Load()
		v := trustedView(rt.b)
		_ = json.NewEncoder(w).Encode(map[string]any{"active": rt.b.Name(), "seg": rt.seg, "last_window": p.lastWindow,
			"table": v.st.Table, "state": v.st.State, "pid": p.pid})
	})
	srv := &http.Server{Addr: *listen, Handler: mux}
	p.j.Append(map[string]any{"kind": "serving", "pid": p.pid, "t": now(), "active": p.cur.Load().b.Name(), "seg": p.cur.Load().seg})
	log.Printf("altun-proxy serving on %s, active %s seg %d", *listen, p.cur.Load().b.Name(), p.cur.Load().seg)
	log.Fatal(srv.ListenAndServe())
}
