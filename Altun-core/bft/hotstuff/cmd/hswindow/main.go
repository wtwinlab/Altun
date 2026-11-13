// hswindow summarises the timed closed-loop measurement window of a hotstuff run.
//
// Each client logs one client-side ThroughputMeasurement (Commands = requests
// completed with f+1 replies inside its window, Commits = requests sent inside
// it, Duration = window length) and a LatencyMeasurement with the same Event
// (mean latency of exactly those requests). This tool reads every given
// measurements.json and prints
//
//	TPS     = sum_i completed_i / window_i
//	latency = (sum_i completed_i * mean_i) / sum_i completed_i
//
// as {"TPS": x, "latency": y} on stdout (the benchmark.log line), and a detailed
// JSON object to -details. If fewer than -clients clients reported, nothing is
// printed on stdout and the exit status is 2.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	_ "github.com/relab/hotstuff/internal/proto/orchestrationpb"
	"github.com/relab/hotstuff/metrics/plotting"
	"github.com/relab/hotstuff/metrics/types"
)

type key struct {
	id uint32
	ts int64
}

type collector struct {
	tput map[key]*types.ThroughputMeasurement
	lat  map[key]*types.LatencyMeasurement
}

func (c *collector) Add(m any) {
	switch v := m.(type) {
	case *types.ThroughputMeasurement:
		if v.GetEvent().GetClient() {
			c.tput[key{v.GetEvent().GetID(), v.GetEvent().GetTimestamp().AsTime().UnixNano()}] = v
		}
	case *types.LatencyMeasurement:
		if v.GetEvent().GetClient() {
			c.lat[key{v.GetEvent().GetID(), v.GetEvent().GetTimestamp().AsTime().UnixNano()}] = v
		}
	}
}

func main() {
	expected := flag.Int("clients", 0, "number of clients expected to report (0 = do not check)")
	details := flag.String("details", "", "file to write a detailed JSON summary to")
	flag.Parse()

	c := &collector{tput: map[key]*types.ThroughputMeasurement{}, lat: map[key]*types.LatencyMeasurement{}}
	var readErrs []string
	for _, p := range flag.Args() {
		f, err := os.Open(p)
		if err != nil {
			readErrs = append(readErrs, err.Error())
			continue
		}
		if err := plotting.NewReader(f, c).ReadAll(); err != nil {
			readErrs = append(readErrs, fmt.Sprintf("%s: %v", p, err))
		}
		f.Close()
	}

	type perClient struct {
		ID        uint32  `json:"id"`
		Window    float64 `json:"window_s"`
		Sent      uint64  `json:"sent"`
		Completed uint64  `json:"completed"`
		Latency   float64 `json:"latency_ms"`
	}
	var (
		clients         []perClient
		tps, latSum     float64
		completed, sent uint64
		minW, maxW      float64
		seen            = map[uint32]bool{}
	)
	for k, t := range c.tput {
		if seen[k.id] {
			readErrs = append(readErrs, fmt.Sprintf("client %d reported twice", k.id))
			continue
		}
		seen[k.id] = true
		w := t.GetDuration().AsDuration().Seconds()
		pc := perClient{ID: k.id, Window: w, Sent: t.GetCommits(), Completed: t.GetCommands()}
		if l, ok := c.lat[k]; ok {
			pc.Latency = l.GetLatency()
		}
		if w > 0 {
			tps += float64(pc.Completed) / w
		}
		latSum += pc.Latency * float64(pc.Completed)
		completed += pc.Completed
		sent += pc.Sent
		if minW == 0 || w < minW {
			minW = w
		}
		if w > maxW {
			maxW = w
		}
		clients = append(clients, pc)
	}
	lat := 0.0
	if completed > 0 {
		lat = latSum / float64(completed)
	}
	ok := *expected == 0 || len(clients) >= *expected
	if *details != "" {
		b, _ := json.MarshalIndent(map[string]any{
			"TPS": tps, "latency": lat, "clients_reported": len(clients), "clients_expected": *expected,
			"completed": completed, "sent": sent, "window_min_s": minW, "window_max_s": maxW,
			"ok": ok, "errors": readErrs, "clients": clients,
		}, "", " ")
		_ = os.WriteFile(*details, append(b, '\n'), 0o644)
	}
	for _, e := range readErrs {
		fmt.Fprintln(os.Stderr, "hswindow:", e)
	}
	if !ok {
		fmt.Fprintf(os.Stderr, "hswindow: only %d of %d clients reported a window\n", len(clients), *expected)
		os.Exit(2)
	}
	fmt.Printf("{\"TPS\": %.6f, \"latency\": %.6f}\n", tps, lat)
}
