// forensic-verify reads one or more forensics log files produced by
// cft-server (via package cft/auditor) and runs a set of verifier
// branches against the collected records, emitting per-branch JSON lines
// to stdout plus a final __summary__ line.
//
// Direction A (2026-04): this binary is the post-hoc audit path. It is
// NOT on the safety critical path — it runs offline, after (or between)
// benchmark epochs. The verifier cannot prevent a violation; it can only
// produce evidence that one happened.
//
// Record format (one JSON object per line; order within the file does
// not matter, the verifier sorts/groups internally):
//
//	{"type":"p1b","replica":"1.1","ballot":4,"log_hash":"<hex>"}
//	{"type":"p2a","replica":"1.2","ballot":4,"slot":17,"cmd_hash":"<hex>"}
//	{"type":"p2b","replica":"1.2","ballot":4,"slot":17,"cmd_hash":"<hex>"}
//	{"type":"commit","replica":"1.1","slot":17,"cmd_hash":"<hex>"}
//
// Verifier branches (all always run; each trace type in Exp 3A is
// designed to exercise exactly one of them):
//
//	p2a_uniqueness    — group p2a rows by (ballot, slot); must have
//	                    |distinct(cmd_hash)| <= 1.
//	p1b_consistency   — group p1b rows by (replica, ballot); must have
//	                    |distinct(log_hash)| <= 1.
//	p2b_uniqueness    — group p2b rows by (replica, ballot, slot); must
//	                    have |distinct(cmd_hash)| <= 1.
//	commit_divergence — group commit rows by slot; must have
//	                    |distinct(cmd_hash)| <= 1.
//
// Exit code: 0 regardless of whether violations were found. A non-zero
// exit code indicates a verifier crash (usage error, unparseable file,
// etc.), which is distinct from "the system under test had a violation".
// Shell callers distinguish the two by checking __summary__.any_violation
// in the output.
//
// Usage:
//
//	forensic-verify <forensics_log_1.jsonl> [forensics_log_2.jsonl ...]
//
// Multiple input files are concatenated, which is how Exp 3A passes the
// per-node forensics logs from a multi-worker cluster run.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

// record is the union type for every row in the forensics log. Fields
// that don't apply to a given type stay zero-valued and are ignored by
// that type's verifier branch.
type record struct {
	Type    string `json:"type"`
	Replica string `json:"replica"`
	Ballot  int    `json:"ballot"`
	Slot    int    `json:"slot"`
	CmdHash string `json:"cmd_hash"`
	LogHash string `json:"log_hash"`
}

// branchResult is what each verifier branch reports back. It is also
// the shape of each output line from this tool.
type branchResult struct {
	Branch         string `json:"branch"`
	ViolationCount int    `json:"violation_count"`
	VerifyMs       int64  `json:"verify_ms"`
	Detail         string `json:"detail,omitempty"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: forensic-verify <log1.jsonl> [log2.jsonl ...]")
		os.Exit(2)
	}

	// Collect all records across all input files.
	var all []record
	for _, path := range os.Args[1:] {
		rs, err := readRecords(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "forensic-verify: cannot read %q: %v\n", path, err)
			os.Exit(2)
		}
		all = append(all, rs...)
	}

	// Run every branch.
	branches := []func([]record) branchResult{
		checkP2aUniqueness,
		checkP1bConsistency,
		checkP2bUniqueness,
		checkCommitDivergence,
	}
	results := make([]branchResult, 0, len(branches))
	totalMs := int64(0)
	anyViolation := false
	for _, fn := range branches {
		r := fn(all)
		results = append(results, r)
		totalMs += r.VerifyMs
		if r.ViolationCount > 0 {
			anyViolation = true
		}
		emit(r)
	}

	// Emit a summary line.
	summary := struct {
		Branch       string `json:"branch"`
		TotalMs      int64  `json:"total_ms"`
		AnyViolation bool   `json:"any_violation"`
		Records      int    `json:"records"`
	}{
		Branch:       "__summary__",
		TotalMs:      totalMs,
		AnyViolation: anyViolation,
		Records:      len(all),
	}
	b, _ := json.Marshal(summary)
	fmt.Println(string(b))
}

// emit prints a branch result as a single JSON line to stdout.
func emit(r branchResult) {
	b, err := json.Marshal(r)
	if err != nil {
		fmt.Fprintf(os.Stderr, "forensic-verify: json marshal failed: %v\n", err)
		return
	}
	fmt.Println(string(b))
}

// readRecords parses a forensics log file. Empty lines are skipped;
// lines that fail to parse as JSON are skipped with a warning on stderr
// (we prefer a partial report to a hard failure, because the purpose of
// this tool is forensics, not gating).
func readRecords(path string) ([]record, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var out []record
	sc := bufio.NewScanner(f)
	// Raise the buffer cap for pathologically long lines.
	buf := make([]byte, 0, 1<<20)
	sc.Buffer(buf, 1<<24)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var r record
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			fmt.Fprintf(os.Stderr, "forensic-verify: %s:%d: bad row: %v\n", path, lineNo, err)
			continue
		}
		out = append(out, r)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// -----------------------------------------------------------------------
// Branch: p2a_uniqueness
// -----------------------------------------------------------------------
// Exp 3A trace `equivocation_p2a` hits this branch.
//
// For every (ballot, slot), every P2a record with that key must agree on
// cmd_hash. A leader that equivocates sends different commands for the
// same (ballot, slot) to different acceptors, which shows up as more than
// one distinct cmd_hash for the grouping key.
//
// We report each offending key as one violation in the count (not each
// extra row), so the number matches reviewer intuition ("how many slots
// were equivocated on?").

func checkP2aUniqueness(all []record) branchResult {
	start := time.Now()
	// key := "ballot:slot" → set of cmd_hash
	seen := map[string]map[string]struct{}{}
	for _, r := range all {
		if r.Type != "p2a" {
			continue
		}
		key := fmt.Sprintf("%d:%d", r.Ballot, r.Slot)
		if seen[key] == nil {
			seen[key] = map[string]struct{}{}
		}
		seen[key][r.CmdHash] = struct{}{}
	}
	var violations []string
	for key, hashes := range seen {
		if len(hashes) > 1 {
			violations = append(violations, fmt.Sprintf(
				"(ballot,slot)=%s has %d distinct cmds", key, len(hashes),
			))
		}
	}
	sort.Strings(violations)
	return branchResult{
		Branch:         "p2a_uniqueness",
		ViolationCount: len(violations),
		VerifyMs:       time.Since(start).Milliseconds(),
		Detail:         strings.Join(violations, "; "),
	}
}

// -----------------------------------------------------------------------
// Branch: p1b_consistency
// -----------------------------------------------------------------------
// Exp 3A trace `equivocation_p1b` hits this branch.
//
// For every (replica, ballot), every P1b record with that key must agree
// on log_hash. An acceptor that gives different "highest accepted" claims
// to different proposers in the same ballot is the classic split-brain
// origin in Paxos.

func checkP1bConsistency(all []record) branchResult {
	start := time.Now()
	seen := map[string]map[string]struct{}{}
	for _, r := range all {
		if r.Type != "p1b" {
			continue
		}
		key := fmt.Sprintf("%s:%d", r.Replica, r.Ballot)
		if seen[key] == nil {
			seen[key] = map[string]struct{}{}
		}
		seen[key][r.LogHash] = struct{}{}
	}
	var violations []string
	for key, hashes := range seen {
		if len(hashes) > 1 {
			violations = append(violations, fmt.Sprintf(
				"(replica,ballot)=%s has %d distinct promise contents", key, len(hashes),
			))
		}
	}
	sort.Strings(violations)
	return branchResult{
		Branch:         "p1b_consistency",
		ViolationCount: len(violations),
		VerifyMs:       time.Since(start).Milliseconds(),
		Detail:         strings.Join(violations, "; "),
	}
}

// -----------------------------------------------------------------------
// Branch: p2b_uniqueness
// -----------------------------------------------------------------------
// Exp 3A trace `vote_flip_p2b` hits this branch.
//
// For every (replica, ballot, slot), that replica must vote for at most
// one cmd. Double-voting is a direct safety break.

func checkP2bUniqueness(all []record) branchResult {
	start := time.Now()
	seen := map[string]map[string]struct{}{}
	for _, r := range all {
		if r.Type != "p2b" {
			continue
		}
		key := fmt.Sprintf("%s:%d:%d", r.Replica, r.Ballot, r.Slot)
		if seen[key] == nil {
			seen[key] = map[string]struct{}{}
		}
		seen[key][r.CmdHash] = struct{}{}
	}
	var violations []string
	for key, hashes := range seen {
		if len(hashes) > 1 {
			violations = append(violations, fmt.Sprintf(
				"(replica,ballot,slot)=%s voted for %d distinct cmds", key, len(hashes),
			))
		}
	}
	sort.Strings(violations)
	return branchResult{
		Branch:         "p2b_uniqueness",
		ViolationCount: len(violations),
		VerifyMs:       time.Since(start).Milliseconds(),
		Detail:         strings.Join(violations, "; "),
	}
}

// -----------------------------------------------------------------------
// Branch: commit_divergence
// -----------------------------------------------------------------------
// Exp 3A trace `commit_divergence` hits this branch.
//
// For every slot, every replica that committed that slot must have
// committed the same cmd. This is the "downstream" observable for any
// upstream equivocation that leaked into the committed log, AND it is
// the direct check for on-disk tampering / log substitution.

func checkCommitDivergence(all []record) branchResult {
	start := time.Now()
	seen := map[int]map[string]struct{}{}
	for _, r := range all {
		if r.Type != "commit" {
			continue
		}
		if seen[r.Slot] == nil {
			seen[r.Slot] = map[string]struct{}{}
		}
		seen[r.Slot][r.CmdHash] = struct{}{}
	}
	var violations []string
	// Collect slots in a stable order for reproducible output.
	slots := make([]int, 0, len(seen))
	for s := range seen {
		slots = append(slots, s)
	}
	sort.Ints(slots)
	for _, s := range slots {
		if len(seen[s]) > 1 {
			violations = append(violations, fmt.Sprintf(
				"slot=%d committed to %d distinct cmds", s, len(seen[s]),
			))
		}
	}
	return branchResult{
		Branch:         "commit_divergence",
		ViolationCount: len(violations),
		VerifyMs:       time.Since(start).Milliseconds(),
		Detail:         strings.Join(violations, "; "),
	}
}
