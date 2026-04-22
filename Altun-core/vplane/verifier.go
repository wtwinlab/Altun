package vplane

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Verifier runs the forensic-verify binary against a forensics log file
// and reports whether any safety violation was detected.
// Implements Algorithm 2 lines 8-9: "check safety of model".
type Verifier struct {
	binPath string // path to forensic-verify binary
}

// NewVerifier creates a verifier that shells out to the given binary.
func NewVerifier(binPath string) *Verifier {
	return &Verifier{binPath: binPath}
}

// VerifyResult holds the outcome of a forensic verification run.
type VerifyResult struct {
	Safe           bool              `json:"safe"`            // true if no violations
	AnyViolation   bool              `json:"any_violation"`   // true if at least one branch found violations
	Records        int               `json:"records"`         // total records verified
	TotalMs        int64             `json:"total_ms"`        // verification wall time
	BranchResults  []BranchResult    `json:"branch_results"`  // per-branch details
	Error          string            `json:"error,omitempty"` // non-empty if verifier crashed
}

// BranchResult mirrors the forensic-verify per-branch output.
type BranchResult struct {
	Branch         string `json:"branch"`
	ViolationCount int    `json:"violation_count"`
	VerifyMs       int64  `json:"verify_ms"`
	Detail         string `json:"detail,omitempty"`
}

// Verify runs forensic-verify on one or more log files and returns the result.
// A 60-second timeout prevents the verifier from blocking reconfiguration.
func (v *Verifier) Verify(ctx context.Context, logPaths ...string) (*VerifyResult, error) {
	if len(logPaths) == 0 {
		return &VerifyResult{Safe: true}, nil
	}

	// Build command: forensic-verify log1.jsonl [log2.jsonl ...]
	args := make([]string, len(logPaths))
	copy(args, logPaths)

	timeout := 60 * time.Second
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, v.binPath, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start forensic-verify: %w", err)
	}

	// Parse JSON lines from stdout
	var branches []BranchResult
	var summary struct {
		Branch       string `json:"branch"`
		TotalMs      int64  `json:"total_ms"`
		AnyViolation bool   `json:"any_violation"`
		Records      int    `json:"records"`
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Try parsing as summary first
		var probe struct {
			Branch string `json:"branch"`
		}
		json.Unmarshal([]byte(line), &probe)

		if probe.Branch == "__summary__" {
			json.Unmarshal([]byte(line), &summary)
		} else {
			var br BranchResult
			if err := json.Unmarshal([]byte(line), &br); err == nil && br.Branch != "" {
				branches = append(branches, br)
			}
		}
	}

	if err := cmd.Wait(); err != nil {
		// Non-zero exit = verifier crash (not a violation)
		return &VerifyResult{
			Safe:          false,
			AnyViolation:  false,
			BranchResults: branches,
			Error:         fmt.Sprintf("forensic-verify exited with error: %v", err),
		}, nil
	}

	return &VerifyResult{
		Safe:          !summary.AnyViolation,
		AnyViolation:  summary.AnyViolation,
		Records:       summary.Records,
		TotalMs:       summary.TotalMs,
		BranchResults: branches,
	}, nil
}
