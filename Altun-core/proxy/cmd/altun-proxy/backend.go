package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/wtwinlab/altunsm"
)

var httpc = &http.Client{Transport: &http.Transport{MaxIdleConnsPerHost: 2000, MaxIdleConns: 10000, IdleConnTimeout: 90 * time.Second}}

// Reply of one member to one op submission.
type Reply struct {
	Member int
	Res    altunsm.Result
	Raw    []byte
	Err    error
}

// Backend is one consensus instance (a protocol over a fixed membership).
type Backend interface {
	Name() string   // paxos | hotstuff
	Family() string // CFT | BFT
	N() int
	F() int
	Admin() []string // base URL per member for /altun/*
	// Exec submits op (attempt = *attempt, *attempt+1, ...) until a result is
	// accepted (CFT: the contacted leader's reply; BFT: f+1 identical replies)
	// or ctx expires.
	Exec(ctx context.Context, op altunsm.Op, tc uint32, base uint64, attempt *uint64) (altunsm.Result, []byte, error)
	// Gather submits op once and returns every member's reply within timeout.
	Gather(op altunsm.Op, tc uint32, tseq uint64, timeout time.Duration) []Reply
	Leader() int // member to try first (CFT: current contact node)
	Rotate(from int)
}

// ---------------------------------------------------------------- Paxos (Paxi)

type paxosBackend struct {
	ids     []string // Paxi ids "1.1".."1.n"
	urls    []string // http addresses
	cur     atomic.Int64
	attempt time.Duration
}

type paxiCmd struct {
	Key       int
	Value     []byte
	ClientID  string
	CommandID int
}

func (b *paxosBackend) Name() string    { return "paxos" }
func (b *paxosBackend) Family() string  { return "CFT" }
func (b *paxosBackend) N() int          { return len(b.urls) }
func (b *paxosBackend) F() int          { return (len(b.urls) - 1) / 2 }
func (b *paxosBackend) Admin() []string { return b.urls }
func (b *paxosBackend) Leader() int     { return int(b.cur.Load()) }
func (b *paxosBackend) Rotate(from int) {
	b.cur.CompareAndSwap(int64(from), int64((from+1)%len(b.urls)))
}

func keyHash(k string) int {
	h := 0
	for i := 0; i < len(k); i++ {
		h = h*31 + int(k[i])
	}
	if h < 0 {
		h = -h
	}
	return h % 1000000
}

func (b *paxosBackend) post(ctx context.Context, member int, op altunsm.Op, tc uint32, tseq uint64) ([]byte, error) {
	v, _ := json.Marshal(op)
	body, _ := json.Marshal(paxiCmd{Key: keyHash(op.K), Value: v, ClientID: "c" + strconv.FormatUint(uint64(tc), 10), CommandID: int(tseq)})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, b.urls[member]+"/", bytes.NewReader(body))
	resp, err := httpc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("paxos %s: %s", resp.Status, bytes.TrimSpace(raw))
	}
	return raw, nil
}

func (b *paxosBackend) Exec(ctx context.Context, op altunsm.Op, tc uint32, base uint64, attempt *uint64) (altunsm.Result, []byte, error) {
	for {
		if ctx.Err() != nil {
			return altunsm.Result{}, nil, ctx.Err()
		}
		m := b.Leader()
		actx, cancel := context.WithTimeout(ctx, b.attempt)
		raw, err := b.post(actx, m, op, tc, base+*attempt)
		cancel()
		*attempt++
		if err == nil {
			var r altunsm.Result
			if json.Unmarshal(raw, &r) == nil {
				return r, raw, nil
			}
			err = errors.New("bad reply")
		}
		b.Rotate(m)
		select {
		case <-time.After(20 * time.Millisecond):
		case <-ctx.Done():
		}
	}
}

func (b *paxosBackend) Gather(op altunsm.Op, tc uint32, tseq uint64, timeout time.Duration) []Reply {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	var at uint64
	r, raw, err := b.Exec(ctx, op, tc, tseq, &at)
	return []Reply{{Member: b.Leader(), Res: r, Raw: raw, Err: err}}
}

// ---------------------------------------------------------------- HotStuff

type hotstuffBackend struct {
	urls    []string // KV front ends
	f       int
	attempt time.Duration
}

func (b *hotstuffBackend) Name() string    { return "hotstuff" }
func (b *hotstuffBackend) Family() string  { return "BFT" }
func (b *hotstuffBackend) N() int          { return len(b.urls) }
func (b *hotstuffBackend) F() int          { return b.f }
func (b *hotstuffBackend) Admin() []string { return b.urls }
func (b *hotstuffBackend) Leader() int     { return 0 }
func (b *hotstuffBackend) Rotate(int)      {}

func (b *hotstuffBackend) post(ctx context.Context, member int, body []byte, tc uint32, tseq uint64) ([]byte, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, b.urls[member]+"/op", bytes.NewReader(body))
	req.Header.Set("Altun-Tc", strconv.FormatUint(uint64(tc), 10))
	req.Header.Set("Altun-Ts", strconv.FormatUint(tseq, 10))
	resp, err := httpc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("hotstuff %s", resp.Status)
	}
	return raw, nil
}

// fanout submits once to every member and streams the replies.
func (b *hotstuffBackend) fanout(ctx context.Context, op altunsm.Op, tc uint32, tseq uint64) <-chan Reply {
	body, _ := json.Marshal(op)
	ch := make(chan Reply, len(b.urls))
	for i := range b.urls {
		go func(i int) {
			raw, err := b.post(ctx, i, body, tc, tseq)
			rp := Reply{Member: i, Raw: raw, Err: err}
			if err == nil {
				if e := json.Unmarshal(raw, &rp.Res); e != nil {
					rp.Err = e
				}
			}
			ch <- rp
		}(i)
	}
	return ch
}

func (b *hotstuffBackend) Exec(ctx context.Context, op altunsm.Op, tc uint32, base uint64, attempt *uint64) (altunsm.Result, []byte, error) {
	for {
		if ctx.Err() != nil {
			return altunsm.Result{}, nil, ctx.Err()
		}
		actx, cancel := context.WithTimeout(ctx, b.attempt)
		ch := b.fanout(actx, op, tc, base+*attempt)
		*attempt++
		counts := map[string]int{}
		for i := 0; i < len(b.urls); i++ {
			rp := <-ch
			if rp.Err != nil || rp.Res.Err == "forked" {
				continue // no result (forked: resubmit under a new transport id)
			}
			k := string(rp.Raw)
			counts[k]++
			if counts[k] >= b.f+1 { // f+1 identical replies: at least one correct replica
				cancel()
				return rp.Res, rp.Raw, nil
			}
		}
		cancel()
	}
}

func (b *hotstuffBackend) Gather(op altunsm.Op, tc uint32, tseq uint64, timeout time.Duration) []Reply {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	ch := b.fanout(ctx, op, tc, tseq)
	out := make([]Reply, 0, len(b.urls))
	for i := 0; i < len(b.urls); i++ {
		out = append(out, <-ch)
	}
	return out
}

// ---------------------------------------------------------------- admin helpers

func getJSON(url string, v any, timeout time.Duration) (http.Header, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	resp, err := httpc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", resp.Status, bytes.TrimSpace(raw))
	}
	return resp.Header, json.Unmarshal(raw, v)
}

func postJSON(url string, v any, timeout time.Duration) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	body, _ := json.Marshal(v)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	resp, err := httpc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return raw, fmt.Errorf("%s: %s", resp.Status, bytes.TrimSpace(raw))
	}
	return raw, nil
}
