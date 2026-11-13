// Package faultinject configures replica-side fault conditions for the BFT
// protocols from the environment. It mirrors the fault hooks of the CFT
// protocols so that both protocol families can be exercised under the same
// conditions:
//
//	SLOW_PROPOSAL_MS  delay, in ms, before each proposal of the replicas in
//	                  BFT_SLOW_IDS (default "1"). With rotating leaders only the
//	                  slow replica's views are delayed (proposal slowness).
//	BFT_ABSENT_IDS    replica IDs that take no part: every proposal, vote, new-view
//	                  and timeout they receive is dropped and they never propose
//	                  (absent replicas).
//
// Read once from the environment of the worker process; `hotstuff run` forwards
// them to every remote worker it starts. Unset means no fault.
package faultinject

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/relab/hotstuff"
)

var (
	slow     time.Duration
	slowIDs  = map[hotstuff.ID]bool{}
	absentID = map[hotstuff.ID]bool{}
)

func parseIDs(v string, into map[hotstuff.ID]bool) {
	for _, s := range strings.Split(v, ",") {
		if n, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
			into[hotstuff.ID(n)] = true
		}
	}
}

func init() {
	if ms, err := strconv.Atoi(os.Getenv("SLOW_PROPOSAL_MS")); err == nil && ms > 0 {
		slow = time.Duration(ms) * time.Millisecond
	}
	ids := os.Getenv("BFT_SLOW_IDS")
	if ids == "" {
		ids = "1"
	}
	parseIDs(ids, slowIDs)
	parseIDs(os.Getenv("BFT_ABSENT_IDS"), absentID)
}

// ProposalDelay sleeps before a proposal by a slow replica.
func ProposalDelay(id hotstuff.ID) {
	if slow > 0 && slowIDs[id] {
		time.Sleep(slow)
	}
}

// IsAbsent reports whether replica id is configured as absent.
func IsAbsent(id hotstuff.ID) bool { return absentID[id] }

// EnvPrefix is the "K=V ..." prefix forwarded to remote workers.
func EnvPrefix() string {
	var sb strings.Builder
	for _, k := range []string{"SLOW_PROPOSAL_MS", "BFT_SLOW_IDS", "BFT_ABSENT_IDS", "HS_WINDOW_MS"} {
		if v := os.Getenv(k); v != "" {
			sb.WriteString(k + "=" + strconv.Quote(v) + " ")
		}
	}
	return sb.String()
}
