package cft

// Fault injection for the CFT protocols. Two fault conditions can be enabled
// through the environment:
//
//   SLOW_PROPOSAL_MS   minimum interval, in ms, between consecutive proposals
//                      sent by the replicas listed in SLOW_PROPOSAL_IDS
//                      (default "1.1"). Proposals are queued and released one
//                      every SLOW_PROPOSAL_MS in order, off the message loop,
//                      so the slow replica's proposal *rate* is throttled —
//                      their "proposal slowness" (interval between consecutive
//                      leader proposals) without stalling its vote handling.
//   ABSENT_IDS         comma-separated replica IDs that do not participate:
//                      every message they receive is dropped, so they never
//                      vote or propose (absentee replicas).
//
// Both are read once from the environment at start-up; apply-cft.sh forwards
// them to every cft-server it launches. Unset means no fault.

import (
	"os"
	"strconv"
	"strings"
	"time"
)

var (
	slowProposal time.Duration
	slowIDs      = map[ID]bool{}
	absentIDs    = map[ID]bool{}
	slowQueue    chan func()
)

func init() {
	if v := os.Getenv("SLOW_PROPOSAL_MS"); v != "" {
		if ms, err := strconv.Atoi(v); err == nil && ms > 0 {
			slowProposal = time.Duration(ms) * time.Millisecond
		}
	}
	ids := os.Getenv("SLOW_PROPOSAL_IDS")
	if ids == "" {
		ids = "1.1"
	}
	for _, s := range strings.Split(ids, ",") {
		if s = strings.TrimSpace(s); s != "" {
			slowIDs[ID(s)] = true
		}
	}
	for _, s := range strings.Split(os.Getenv("ABSENT_IDS"), ",") {
		if s = strings.TrimSpace(s); s != "" {
			absentIDs[ID(s)] = true
		}
	}
	if slowProposal > 0 {
		slowQueue = make(chan func(), 1<<16)
		go func() {
			for send := range slowQueue {
				send()
				time.Sleep(slowProposal)
			}
		}()
	}
}

// SendProposal sends a proposal, throttled if this replica is configured as
// slow. Called by every protocol at the point it sends a proposal (paxos P2a,
// epaxos PreAccept, vpaxos zone P2a; wpaxos goes through paxos).
func SendProposal(id ID, send func()) {
	if slowProposal > 0 && slowIDs[id] {
		slowQueue <- send
		return
	}
	send()
}

// IsAbsent reports whether id is configured as an absentee replica.
func IsAbsent(id ID) bool {
	return absentIDs[id]
}
