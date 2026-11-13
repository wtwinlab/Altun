package replica

// Altun KV front end of a HotStuff replica.
//
// External clients reach the BFT instance only through the Altun proxy.  The
// proxy POSTs every op to /op on every replica; the replica turns it into a
// HotStuff client command (transport id = the proxy's (client, attempt-seq)),
// puts it into its command cache, and answers once the op has been executed
// by the replicated KV state machine (altunsm) in commit order.  The proxy
// accepts a result once f+1 replicas returned byte-identical results.

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/relab/hotstuff/internal/proto/clientpb"
	"github.com/wtwinlab/altunsm"
)

type kvFront struct {
	app     *altunsm.SM
	srv     *clientSrv
	mut     sync.Mutex
	waiters map[cmdID]chan []byte
	timeout time.Duration
}

// exec is called by clientSrv.Exec, in commit order, on the event loop.
func (f *kvFront) exec(cmd *clientpb.Command) {
	res := f.app.Apply(cmd.GetData())
	id := cmdID{cmd.GetClientID(), cmd.GetSequenceNumber()}
	f.mut.Lock()
	if c, ok := f.waiters[id]; ok {
		c <- res
		delete(f.waiters, id)
	}
	f.mut.Unlock()
}

// fork is called for commands of a block that was pruned without being
// committed: the command will never execute under this transport id (the
// command cache already counts it as proposed), so the waiter is told to
// resubmit under a new id instead of waiting for its timeout.
func (f *kvFront) fork(cmd *clientpb.Command) {
	id := cmdID{cmd.GetClientID(), cmd.GetSequenceNumber()}
	f.mut.Lock()
	if c, ok := f.waiters[id]; ok {
		c <- []byte(`{"ok":false,"err":"forked","seg":0}`)
		delete(f.waiters, id)
	}
	f.mut.Unlock()
}

func (f *kvFront) handleOp(w http.ResponseWriter, r *http.Request) {
	cid, err1 := strconv.ParseUint(r.Header.Get("Altun-Tc"), 10, 32)
	tseq, err2 := strconv.ParseUint(r.Header.Get("Altun-Ts"), 10, 64)
	body, err3 := io.ReadAll(r.Body)
	if err1 != nil || err2 != nil || err3 != nil {
		http.Error(w, "bad transport id", http.StatusBadRequest)
		return
	}
	id := cmdID{uint32(cid), tseq}
	c := make(chan []byte, 1)
	f.mut.Lock()
	f.waiters[id] = c
	f.mut.Unlock()
	f.srv.cmdCache.addCommand(&clientpb.Command{ClientID: uint32(cid), SequenceNumber: tseq, Data: body})
	t := time.NewTimer(f.timeout)
	defer t.Stop()
	select {
	case res := <-c:
		_, _ = w.Write(res)
	case <-t.C:
		f.mut.Lock()
		delete(f.waiters, id)
		f.mut.Unlock()
		http.Error(w, "timeout", http.StatusGatewayTimeout)
	case <-r.Context().Done():
		f.mut.Lock()
		delete(f.waiters, id)
		f.mut.Unlock()
	}
}

// StartKVFront enables the Altun KV state machine on this replica and serves
// the proxy-facing HTTP API on addr.  idle is the empty-proposal interval.
func (srv *Replica) StartKVFront(addr string, app *altunsm.SM, idle, timeout time.Duration) error {
	f := &kvFront{app: app, srv: srv.clientSrv, waiters: map[cmdID]chan []byte{}, timeout: timeout}
	srv.clientSrv.kv = f
	srv.clientSrv.cmdCache.idle = idle
	mux := http.NewServeMux()
	mux.HandleFunc("/op", f.handleOp)
	app.RegisterHTTP(mux)
	mux.HandleFunc("/altun/ping", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	})
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	go func() { _ = http.Serve(lis, mux) }()
	return nil
}
