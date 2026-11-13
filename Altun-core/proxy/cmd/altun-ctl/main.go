// altun-ctl is the controller side: key generation, signing policy-window
// directives, and periodic switch schedules.
//
//	altun-ctl -genkeys DIR/NAME                      (writes NAME.key / NAME.pub; also used for replica attestation keys)
//	altun-ctl -key DIR/ctl.key -proxy URL -to hotstuff -window 7 [-f 1 -n 4 -fb 1]
//	altun-ctl -key DIR/ctl.key -proxy URL -every 1s -count 40 -window 1 -first hotstuff
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/wtwinlab/altunproxy/directive"
)

func main() {
	genkeys := flag.String("genkeys", "", "write a key pair to PATH.key / PATH.pub and exit")
	key := flag.String("key", "", "private key")
	proxy := flag.String("proxy", "http://127.0.0.1:37400", "proxy")
	to := flag.String("to", "hotstuff", "target instance (single switch)")
	window := flag.Uint64("window", 1, "policy window id (first one for a schedule)")
	f := flag.Int("f", 1, "fault threshold")
	n := flag.Int("n", 4, "membership size")
	fb := flag.Int("fb", -1, "certification fault bound f_B carried by the directive (-1: omit, the proxy uses f; 0: unanimity)")
	every := flag.Duration("every", 0, "schedule: switch period (0 = single switch)")
	count := flag.Int("count", 0, "schedule: number of switches")
	first := flag.String("first", "hotstuff", "schedule: first target; then alternate")
	out := flag.String("out", "", "append reports (JSONL) here")
	flag.Parse()
	if *genkeys != "" {
		if err := directive.GenKeys(*genkeys); err != nil {
			log.Fatal(err)
		}
		return
	}
	priv, err := directive.LoadPriv(*key)
	if err != nil {
		log.Fatal(err)
	}
	var w io.Writer = os.Stdout
	if *out != "" {
		fh, err := os.OpenFile(*out, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			log.Fatal(err)
		}
		defer fh.Close()
		w = io.MultiWriter(os.Stdout, fh)
	}
	send := func(win uint64, target string) {
		d := directive.Directive{Window: win, To: target, F: *f, N: *n, Ts: time.Now().UnixNano()}
		if *fb >= 0 {
			v := *fb
			d.FB = &v
		}
		d.Sign(priv)
		b, _ := json.Marshal(d)
		resp, err := http.Post(*proxy+"/admin/switch", "application/json", bytes.NewReader(b))
		if err != nil {
			fmt.Fprintf(w, "{\"error\":%q,\"window\":%d}\n", err.Error(), win)
			return
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		fmt.Fprintf(w, "%s", bytes.TrimSpace(body))
		fmt.Fprintln(w)
	}
	if *every == 0 {
		send(*window, *to)
		return
	}
	tick := time.NewTicker(*every)
	defer tick.Stop()
	for i := 0; i < *count; i++ {
		<-tick.C
		// target = the instance that is not active now (alternation survives aborts)
		var st struct {
			Active string `json:"active"`
		}
		target := *first
		if resp, err := http.Get(*proxy + "/admin/status"); err == nil {
			_ = json.NewDecoder(resp.Body).Decode(&st)
			resp.Body.Close()
			if st.Active == "hotstuff" {
				target = "paxos"
			} else if st.Active == "paxos" {
				target = "hotstuff"
			}
		}
		send(*window+uint64(i), target)
	}
}
