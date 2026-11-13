// altun-check checks a merged client history (altun-load JSONL files, common
// clock) recorded across policy-window switches.
//
//  1. Linearizability, independent of anything the system reports: per key, a
//     Wing-Gong-Lowe search (as in Porcupine) for a read/write register.
//     Pending puts may take effect at any point after their invocation;
//     pending gets are dropped.
//  2. Virtual-log witness, using the global positions the replicas returned:
//     no position is used by two different ops (agreement across segments),
//     positions respect real-time order, every get returns the latest put
//     before it in position order, and every op's position lies inside the
//     segment it reports (segment boundaries from the proxy journal).
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"hash/fnv"
	"log"
	"math"
	"os"
	"sort"
	"strings"
)

type Rec struct {
	C     uint64 `json:"c"`
	S     uint64 `json:"s"`
	T     string `json:"t"`
	K     string `json:"k"`
	V     string `json:"v"`
	Found bool   `json:"found"`
	Prev  string `json:"p"`
	Inv   int64  `json:"inv"`
	Ret   int64  `json:"ret"`
	Pos   uint64 `json:"pos"`
	Seg   uint64 `json:"seg"`
}

// ------------------------------------------------------------ WGL per key

type op struct {
	id       int
	put      bool
	v        string
	found    bool
	inv, ret int64
}

type node struct {
	op         *op
	call       bool
	match      *node
	prev, next *node
	t          int64
}

const absent = "\x00absent"

func step(state string, o *op) (bool, string) {
	if o.put {
		return true, o.v
	}
	if !o.found {
		return state == absent, state
	}
	return state == o.v, state
}

type bitset []uint64

func (b bitset) set(i int)   { b[i/64] |= 1 << (uint(i) % 64) }
func (b bitset) clear(i int) { b[i/64] &^= 1 << (uint(i) % 64) }
func (b bitset) clone() bitset {
	c := make(bitset, len(b))
	copy(c, b)
	return c
}
func (b bitset) hash() uint64 {
	h := fnv.New64a()
	for _, w := range b {
		var x [8]byte
		for i := 0; i < 8; i++ {
			x[i] = byte(w >> (8 * i))
		}
		h.Write(x[:])
	}
	return h.Sum64()
}
func (b bitset) eq(c bitset) bool {
	for i := range b {
		if b[i] != c[i] {
			return false
		}
	}
	return true
}

func lift(e *node) {
	e.prev.next = e.next
	e.next.prev = e.prev
	m := e.match
	m.prev.next = m.next
	if m.next != nil {
		m.next.prev = m.prev
	}
}

func unlift(e *node) {
	m := e.match
	m.prev.next = m
	if m.next != nil {
		m.next.prev = m
	}
	e.prev.next = e
	e.next.prev = e
}

type cacheEntry struct {
	lin   bitset
	state string
}

// checkKey returns (linearizable, explored-all).
func checkKey(ops []*op, limit int) (bool, bool) {
	var nodes []*node
	for _, o := range ops {
		c := &node{op: o, call: true, t: o.inv}
		r := &node{op: o, t: o.ret}
		c.match = r
		nodes = append(nodes, c, r)
	}
	sort.SliceStable(nodes, func(i, j int) bool {
		if nodes[i].t != nodes[j].t {
			return nodes[i].t < nodes[j].t
		}
		return nodes[i].call && !nodes[j].call // calls first on ties (conservative)
	})
	head := &node{}
	prev := head
	for _, n := range nodes {
		prev.next = n
		n.prev = prev
		prev = n
	}
	type frame struct {
		e     *node
		state string
	}
	var stack []frame
	lin := make(bitset, (len(ops)+63)/64)
	cache := map[uint64][]cacheEntry{}
	state := absent
	e := head.next
	steps := 0
	for head.next != nil {
		steps++
		if steps > limit {
			return true, false
		}
		if e.call {
			ok, ns := step(state, e.op)
			if ok {
				nl := lin.clone()
				nl.set(e.op.id)
				h := nl.hash() ^ uint64(len(ns))*1000003
				seen := false
				for _, ce := range cache[h] {
					if ce.state == ns && ce.lin.eq(nl) {
						seen = true
						break
					}
				}
				if !seen {
					cache[h] = append(cache[h], cacheEntry{nl, ns})
					stack = append(stack, frame{e, state})
					state = ns
					lin.set(e.op.id)
					lift(e)
					e = head.next
					continue
				}
			}
			e = e.next
		} else {
			if len(stack) == 0 {
				return false, true
			}
			top := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			state = top.state
			lin.clear(top.e.op.id)
			unlift(top.e)
			e = top.e.next
		}
	}
	return true, true
}

// ------------------------------------------------------------ segments

type seg struct {
	id          uint64
	start, tail uint64 // tail = MaxUint64 while open
	inst        string
}

func readSegments(path string) (map[uint64]*seg, []string) {
	segs := map[uint64]*seg{}
	var notes []string
	if path == "" {
		return segs, notes
	}
	f, err := os.Open(path)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<26)
	for sc.Scan() {
		var m map[string]any
		if json.Unmarshal(sc.Bytes(), &m) != nil {
			continue
		}
		num := func(k string) uint64 { v, _ := m[k].(float64); return uint64(v) }
		switch m["kind"] {
		case "replay":
			// segment table replayed from the replicated state at proxy (re)start
			var tb struct {
				Table []struct {
					Kind  string `json:"kind"`
					Seg   uint64 `json:"seg"`
					Tail  uint64 `json:"tail"`
					Start uint64 `json:"start"`
					To    string `json:"to"`
				} `json:"table"`
			}
			_ = json.Unmarshal(sc.Bytes(), &tb)
			for _, r := range tb.Table {
				switch r.Kind {
				case "activate", "abort":
					if _, ok := segs[r.Seg]; !ok {
						segs[r.Seg] = &seg{id: r.Seg, start: r.Start, tail: math.MaxUint64, inst: r.To}
					}
				case "seal":
					if s, ok := segs[r.Seg]; ok {
						s.tail = r.Tail
					}
				}
			}
		case "bootstrap":
			segs[1] = &seg{id: 1, start: 1, tail: math.MaxUint64, inst: fmt.Sprint(m["to"])}
		case "switch":
			if ok, _ := m["ok"].(bool); !ok && !(m["aborted"] == true && num("new_seg") > 0) {
				continue
			}
			old, nw, tail := num("old_seg"), num("new_seg"), num("tail")
			if s, ok := segs[old]; ok {
				s.tail = tail
			}
			inst := fmt.Sprint(m["to"])
			if m["aborted"] == true {
				inst = fmt.Sprint(m["from"])
				segs[old+1] = &seg{id: old + 1, start: tail + 1, tail: tail, inst: "aborted"}
			}
			segs[nw] = &seg{id: nw, start: tail + 1, tail: math.MaxUint64, inst: inst}
			if m["unsafe"] != nil {
				notes = append(notes, fmt.Sprintf("segment %d created by unsafe switch (%v)", nw, m["unsafe"]))
			}
		}
	}
	return segs, notes
}

// ------------------------------------------------------------ main

func main() {
	journal := flag.String("journal", "", "proxy journal (segments.jsonl)")
	out := flag.String("out", "", "write the JSON summary here too")
	limit := flag.Int("limit", 50_000_000, "WGL step limit per key")
	maxShow := flag.Int("show", 10, "violations to print")
	metaOut := flag.String("export-meta", "", "also write the segment events (activate/seal with their positions) as JSONL, derived from the journal")
	csvOut := flag.String("export-csv", "", "also write the history as CSV (client,op,key,value,invoke_ns,response_ns,ok,result,segment,vlog_index) for external checkers")
	flag.Parse()

	var recs []Rec
	for _, p := range flag.Args() {
		f, err := os.Open(p)
		if err != nil {
			log.Fatal(err)
		}
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 1<<20), 1<<24)
		for sc.Scan() {
			var r Rec
			if err := json.Unmarshal(sc.Bytes(), &r); err == nil {
				recs = append(recs, r)
			}
		}
		f.Close()
	}
	sum := map[string]any{"ops": len(recs)}
	if *csvOut != "" {
		exportCSV(*csvOut, recs)
	}
	var viol []string

	// ---- 1. linearizability per key
	byKey := map[string][]*op{}
	pending, dropped := 0, 0
	for _, r := range recs {
		o := &op{put: r.T == "put", v: r.V, found: r.Found, inv: r.Inv, ret: r.Ret}
		if r.Ret == 0 {
			pending++
			if !o.put {
				dropped++
				continue
			}
			o.ret = math.MaxInt64
		}
		o.id = len(byKey[r.K])
		byKey[r.K] = append(byKey[r.K], o)
	}
	keys := make([]string, 0, len(byKey))
	for k := range byKey {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	badKeys, unknownKeys := 0, 0
	for _, k := range keys {
		ok, full := checkKey(byKey[k], *limit)
		if !full {
			unknownKeys++
		} else if !ok {
			badKeys++
			if len(viol) < *maxShow {
				viol = append(viol, fmt.Sprintf("key %s: history not linearizable (%d ops)", k, len(byKey[k])))
			}
		}
	}
	sum["keys"] = len(keys)
	sum["pending"] = pending
	sum["lin_violating_keys"] = badKeys
	sum["lin_unknown_keys"] = unknownKeys

	// ---- 2. virtual-log witness
	var done []Rec
	for _, r := range recs {
		if r.Ret != 0 && r.Pos > 0 {
			done = append(done, r)
		}
	}
	sort.Slice(done, func(i, j int) bool { return done[i].Pos < done[j].Pos })
	dupPos, rtViol, readViol, segViol, unverifiable := 0, 0, 0, 0, 0
	for i := 1; i < len(done); i++ {
		if done[i].Pos == done[i-1].Pos {
			dupPos++
			if len(viol) < *maxShow {
				viol = append(viol, fmt.Sprintf("position %d used by two ops: c%d.%d (seg %d) and c%d.%d (seg %d)", done[i].Pos, done[i-1].C, done[i-1].S, done[i-1].Seg, done[i].C, done[i].S, done[i].Seg))
			}
		}
	}
	// real time: if A returned before B was invoked then pos(A) < pos(B)
	minRet := int64(math.MaxInt64)
	for i := len(done) - 1; i >= 0; i-- {
		if minRet < done[i].Inv {
			rtViol++
			if len(viol) < *maxShow {
				viol = append(viol, fmt.Sprintf("real-time order: op at pos %d invoked after an op at a higher position returned", done[i].Pos))
			}
		}
		if done[i].Ret < minRet {
			minRet = done[i].Ret
		}
	}
	pendingPut := map[string]bool{}
	for _, r := range recs {
		if r.Ret == 0 && r.T == "put" {
			pendingPut[r.K+"\x00"+r.V] = true
		}
	}
	last := map[string]string{}
	for _, r := range done {
		if r.T == "put" {
			last[r.K] = r.V
			continue
		}
		want, has := last[r.K]
		if r.Found && pendingPut[r.K+"\x00"+r.V] {
			unverifiable++
			last[r.K] = r.V
			continue
		}
		if r.Found != has || (has && r.V != want) {
			readViol++
			if len(viol) < *maxShow {
				viol = append(viol, fmt.Sprintf("get %s at pos %d (seg %d) returned %q/%v, latest put in position order is %q/%v", r.K, r.Pos, r.Seg, r.V, r.Found, want, has))
			}
		}
	}
	segs, notes := readSegments(*journal)
	if *metaOut != "" {
		f, err := os.Create(*metaOut)
		if err != nil {
			log.Fatal(err)
		}
		ids := make([]uint64, 0, len(segs))
		for id := range segs {
			ids = append(ids, id)
		}
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
		for _, id := range ids {
			sg := segs[id]
			if sg.inst == "aborted" {
				continue
			}
			fmt.Fprintf(f, "{\"event\":\"activate\",\"segment\":%d,\"start_index\":%d,\"protocol\":%q}\n", id, sg.start, sg.inst)
			if sg.tail != math.MaxUint64 {
				fmt.Fprintf(f, "{\"event\":\"seal\",\"segment\":%d,\"seal_index\":%d,\"protocol\":%q}\n", id, sg.tail, sg.inst)
			}
		}
		f.Close()
	}
	if len(segs) > 0 {
		for _, r := range done {
			s, ok := segs[r.Seg]
			if !ok || r.Pos < s.start || r.Pos > s.tail {
				segViol++
				if len(viol) < *maxShow {
					viol = append(viol, fmt.Sprintf("op c%d.%d at pos %d reports seg %d outside its range", r.C, r.S, r.Pos, r.Seg))
				}
			}
		}
	}
	sum["witness_ops"] = len(done)
	sum["dup_positions"] = dupPos
	sum["realtime_violations"] = rtViol
	sum["read_violations"] = readViol
	sum["segment_violations"] = segViol
	sum["reads_of_pending_puts"] = unverifiable
	sum["segments"] = len(segs)
	sum["notes"] = notes
	total := badKeys + dupPos + rtViol + readViol + segViol
	sum["violations_total"] = total
	sum["examples"] = viol
	b, _ := json.MarshalIndent(sum, "", "  ")
	fmt.Println(string(b))
	if *out != "" {
		_ = os.WriteFile(*out, b, 0o644)
	}
	if total > 0 {
		fmt.Fprintln(os.Stderr, "VIOLATIONS:", strings.Join(viol, "\n  "))
		os.Exit(1)
	}
}

// exportCSV writes the history in a flat CSV format consumable by external
// linearizability checkers.  Keys "k<n>" map to n; a missing value is "".
func exportCSV(path string, recs []Rec) {
	f, err := os.Create(path)
	if err != nil {
		log.Fatal(err)
	}
	w := bufio.NewWriter(f)
	for _, r := range recs {
		k := strings.TrimPrefix(r.K, "k")
		op, val, res := "r", "", r.V
		if r.T == "put" {
			op, val, res = "w", r.V, r.Prev
		}
		ok := 1
		if r.Ret == 0 {
			ok = 0
		}
		fmt.Fprintf(w, "c%d,%s,%s,%s,%d,%d,%d,%s,%d,%d\n", r.C, op, k, val, r.Inv, r.Ret, ok, res, r.Seg, r.Pos)
	}
	w.Flush()
	f.Close()
}
