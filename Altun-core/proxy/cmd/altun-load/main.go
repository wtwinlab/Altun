// altun-load is a closed-loop KV client population that talks only to the
// Altun proxy and records a per-op history with a common clock (all
// processes run on one host; wall-clock ns).  Every put writes a unique
// value; an op is retried with the same (client, seq) until it succeeds, and
// its invocation time stays the time of the first attempt.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/wtwinlab/altunsm"
)

// Rec is one history record.
type Rec struct {
	C     uint64 `json:"c"`
	S     uint64 `json:"s"`
	T     string `json:"t"`
	K     string `json:"k"`
	V     string `json:"v,omitempty"` // put: value written; get: value read
	Found bool   `json:"found,omitempty"`
	Prev  string `json:"p,omitempty"` // put: overwritten value
	Inv   int64  `json:"inv"`
	Ret   int64  `json:"ret"` // 0 = no response (pending)
	Pos   uint64 `json:"pos,omitempty"`
	Seg   uint64 `json:"seg,omitempty"`
	Att   int    `json:"att"`
	Dup   bool   `json:"dup,omitempty"`
	Err   string `json:"err,omitempty"`
}

func main() {
	proxy := flag.String("proxy", "http://127.0.0.1:37400", "proxy")
	clients := flag.Int("clients", 16, "closed-loop clients")
	base := flag.Uint64("cid-base", 1000, "first client id")
	keys := flag.Int("keys", 100, "key space")
	writes := flag.Float64("writes", 0.5, "fraction of puts")
	dur := flag.Duration("duration", 10*time.Second, "issue new ops for this long")
	grace := flag.Duration("grace", 60*time.Second, "max time to finish outstanding ops after duration")
	out := flag.String("out", "history.jsonl", "history output")
	seed := flag.Int64("seed", 1, "rng seed")
	flag.Parse()

	tr := &http.Transport{MaxIdleConnsPerHost: 4096}
	hc := &http.Client{Transport: tr, Timeout: 70 * time.Second}
	start := time.Now()
	stopIssue := start.Add(*dur)
	hardStop := stopIssue.Add(*grace)
	var mu sync.Mutex
	var all []Rec
	var wg sync.WaitGroup
	for i := 0; i < *clients; i++ {
		wg.Add(1)
		go func(cid uint64) {
			defer wg.Done()
			rng := rand.New(rand.NewSource(*seed*7919 + int64(cid)))
			var hist []Rec
			for seq := uint64(1); time.Now().Before(stopIssue); seq++ {
				op := altunsm.Op{C: cid, S: seq, K: "k" + strconv.Itoa(rng.Intn(*keys))}
				if rng.Float64() < *writes {
					op.T, op.V = "put", fmt.Sprintf("%d.%d", cid, seq)
				} else {
					op.T = "get"
				}
				body, _ := json.Marshal(op)
				rec := Rec{C: cid, S: seq, T: op.T, K: op.K, V: op.V, Inv: time.Now().UnixNano()}
				for att := 0; ; att++ {
					if time.Now().After(hardStop) {
						rec.Err = "pending"
						break
					}
					req, _ := http.NewRequest(http.MethodPost, *proxy+"/kv", bytes.NewReader(body))
					req.Header.Set("Altun-Attempt", strconv.Itoa(att))
					resp, err := hc.Do(req)
					if err == nil {
						raw, _ := io.ReadAll(resp.Body)
						resp.Body.Close()
						if resp.StatusCode == http.StatusOK {
							var r altunsm.Result
							if json.Unmarshal(raw, &r) == nil && r.OK {
								rec.Ret = time.Now().UnixNano()
								rec.Pos, rec.Seg, rec.Dup, rec.Att = r.Pos, r.Seg, r.Dup, att
								if op.T == "get" {
									rec.V, rec.Found = r.V, r.Found
								} else {
									rec.Prev = r.Prev
								}
								break
							}
							rec.Err = string(raw)
						}
					}
					time.Sleep(20 * time.Millisecond)
				}
				hist = append(hist, rec)
				if rec.Ret == 0 {
					break // pending op: this client stops (its outcome is unknown)
				}
			}
			mu.Lock()
			all = append(all, hist...)
			mu.Unlock()
		}(*base + uint64(i))
	}
	wg.Wait()
	f, err := os.Create(*out)
	if err != nil {
		log.Fatal(err)
	}
	w := bufio.NewWriter(f)
	enc := json.NewEncoder(w)
	pending := 0
	for _, r := range all {
		if r.Ret == 0 {
			pending++
		}
		_ = enc.Encode(r)
	}
	w.Flush()
	f.Close()
	el := time.Since(start).Seconds()
	log.Printf("altun-load: %d ops (%d pending) in %.1fs by %d clients", len(all), pending, el, *clients)
}
