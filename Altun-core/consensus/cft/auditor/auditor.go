// Package auditor writes CFT consensus messages (P1b promises, P2a proposals,
// P2b accept votes, and commit events) to an append-only forensics log file.
// An out-of-process verifier (cmd/forensic-verify) reads that log between
// epochs and flags safety-violating patterns: equivocation at P1b/P2a/P2b,
// and cross-replica commit divergence.
//
// Direction A (2026-04): this package is a POST-HOC forensic log collector.
// It does NOT detect byzantine behaviour in real time, it does NOT write
// b.env, and it does NOT trigger any mode switching. A failure of the
// auditor does not degrade the safety of the underlying consensus protocol;
// the only consequence is that forensic evidence for the affected epoch is
// lost.
//
// Environment variables:
//
//	FORENSICS_LOG=<path>  – path to the forensics log file. If unset, all
//	                        Log* functions are cheap no-ops. The path must be
//	                        writable by every cft-server process that will
//	                        call into this package — for multi-node clusters
//	                        this means an NFS path.
//
// Legacy env vars (AUDITOR_ENABLED / BENV_PATH / the in-memory Observe
// detector) from the pre-Direction-A framing are gone. If you still see
// references in shell scripts, they are no-ops in this build.
package auditor

import (
	"bufio"
	"fmt"
	"os"
	"sync"
)

var (
	mu      sync.Mutex
	fh      *os.File
	wr      *bufio.Writer
	enabled bool
)

// Init reads FORENSICS_LOG and, if set, opens the file in append mode.
// Call once from cft-server main() before any protocol message handling.
// Safe to call with FORENSICS_LOG unset; in that case all Log* calls below
// return immediately.
func Init() {
	path := os.Getenv("FORENSICS_LOG")
	if path == "" {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		// Fall through to disabled. We deliberately do not return an error
		// or panic: a missing forensics log must never break consensus.
		fmt.Fprintf(os.Stderr, "auditor: cannot open FORENSICS_LOG=%q: %v (disabling)\n", path, err)
		return
	}
	fh = f
	wr = bufio.NewWriter(f)
	enabled = true
}

// Flush pushes any buffered rows to disk. Call before process exit if you
// want to avoid losing the last few events. Safe to call when disabled.
func Flush() {
	mu.Lock()
	defer mu.Unlock()
	if wr != nil {
		_ = wr.Flush()
	}
	if fh != nil {
		_ = fh.Sync()
	}
}

// writeJSON appends one JSON row to the forensics log.
// Called only by the Log* functions below; takes the mutex itself.
func writeJSON(line string) {
	if !enabled {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	if wr == nil {
		return
	}
	_, _ = wr.WriteString(line)
	_, _ = wr.WriteString("\n")
	// Auto-flush every record so a crash doesn't lose the last epoch's
	// worth of audit evidence. This is cheap relative to consensus
	// latency (Paxos P2a->P2b already does a network round-trip).
	_ = wr.Flush()
}

// LogP1b records that replica r emitted (or received, depending on where
// the hook is placed) a P1b promise in the given ballot with the given
// "log hash" (a deterministic digest of the "highest accepted per slot"
// map). Used by the verifier's p1b_consistency branch to detect acceptors
// that send conflicting promises to different proposers.
func LogP1b(replica string, ballot int, logHash string) {
	if !enabled {
		return
	}
	writeJSON(fmt.Sprintf(
		`{"type":"p1b","replica":%q,"ballot":%d,"log_hash":%q}`,
		replica, ballot, logHash,
	))
}

// LogP2a records that replica r handled a P2a message with the given
// (ballot, slot, cmdHash) tuple. Used by the verifier's p2a_uniqueness
// branch: if two replicas log different cmdHash for the same (ballot, slot),
// the leader equivocated.
func LogP2a(replica string, ballot int, slot int, cmdHash string) {
	if !enabled {
		return
	}
	writeJSON(fmt.Sprintf(
		`{"type":"p2a","replica":%q,"ballot":%d,"slot":%d,"cmd_hash":%q}`,
		replica, ballot, slot, cmdHash,
	))
}

// LogP2b records that replica r cast a P2b accept vote for the given
// (ballot, slot, cmdHash) tuple. Used by the verifier's p2b_uniqueness
// branch: any acceptor must vote at most once per (ballot, slot).
func LogP2b(voter string, ballot int, slot int, cmdHash string) {
	if !enabled {
		return
	}
	writeJSON(fmt.Sprintf(
		`{"type":"p2b","replica":%q,"ballot":%d,"slot":%d,"cmd_hash":%q}`,
		voter, ballot, slot, cmdHash,
	))
}

// LogCommit records that replica r locally committed slot with the given
// cmdHash. Used by the verifier's commit_divergence branch: for every slot,
// all replicas that committed it must agree on cmdHash.
func LogCommit(replica string, slot int, cmdHash string) {
	if !enabled {
		return
	}
	writeJSON(fmt.Sprintf(
		`{"type":"commit","replica":%q,"slot":%d,"cmd_hash":%q}`,
		replica, slot, cmdHash,
	))
}

// Observe is retained as a thin compatibility shim for any callers that
// still use the pre-Direction-A API. It is a no-op wrapper around LogP2a
// so existing tests and any un-migrated call sites keep compiling. New
// code should call LogP2a directly.
//
// Deprecated: use LogP2a.
func Observe(ballot, slot int, command string) bool {
	// We don't know the replica ID here (the old API never captured it),
	// so we tag the row with an empty replica field. The verifier still
	// handles those records correctly for the p2a_uniqueness branch,
	// because grouping is by (ballot, slot) not (replica, ballot, slot).
	LogP2a("", ballot, slot, command)
	return false
}
