// Package adapter provides a gRPC ConsensusInstance server that manages a
// BFT protocol (hotstuff) as a subprocess. The adapter translates gRPC calls
// into process lifecycle operations (start, SIGTERM, metrics parsing).
package adapter

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/wtwinlab/altun/proto/altunpb"
)

// BFTConfig holds parameters for launching the bft-server subprocess.
type BFTConfig struct {
	BinPath     string // path to bft-server binary
	Protocol    string // chainedhotstuff, fasthotstuff, etc.
	Replicas    int
	Clients     int
	BatchSize   int
	PayloadSize int
	RateLimit   int
	Duration    int    // seconds
	OutputDir   string // measurement output directory
}

// BFTAdapter implements gRPC ConsensusInstance by wrapping bft-server process.
type BFTAdapter struct {
	altunpb.UnimplementedConsensusInstanceServer

	id     int64
	config BFTConfig
	cmd    *exec.Cmd
	sealed atomic.Bool
}

// NewBFTAdapter creates a new adapter. Call Start() to launch the subprocess.
func NewBFTAdapter(id int64, cfg BFTConfig) *BFTAdapter {
	return &BFTAdapter{id: id, config: cfg}
}

// Start launches the bft-server subprocess.
func (a *BFTAdapter) Start() error {
	a.cmd = exec.Command(a.config.BinPath, "run",
		"--consensus", a.config.Protocol,
		"--replicas", fmt.Sprintf("%d", a.config.Replicas),
		"--clients", fmt.Sprintf("%d", a.config.Clients),
		"--batch-size", fmt.Sprintf("%d", a.config.BatchSize),
		"--payload-size", fmt.Sprintf("%d", a.config.PayloadSize),
		"--rate-limit", fmt.Sprintf("%d", a.config.RateLimit),
		"--duration", fmt.Sprintf("%ds", a.config.Duration),
		"--output", a.config.OutputDir,
	)
	a.cmd.Stdout = os.Stdout
	a.cmd.Stderr = os.Stderr
	return a.cmd.Start()
}

// Propose is not supported for BFT subprocess (uses internal client).
func (a *BFTAdapter) Propose(ctx context.Context, req *altunpb.ProposeRequest) (*altunpb.ProposeResponse, error) {
	return &altunpb.ProposeResponse{Error: "bft adapter: propose not supported, uses internal client"}, nil
}

// LastCommitted parses final metrics from bft-performance.json.
func (a *BFTAdapter) LastCommitted(ctx context.Context, _ *altunpb.Empty) (*altunpb.CommitStatus, error) {
	idx := a.parseFinalIndex()
	h := sha256.Sum256([]byte(fmt.Sprintf("bft-%d-%d", a.id, idx)))
	return &altunpb.CommitStatus{Index: idx, StateHash: h[:]}, nil
}

// Checkpoint returns a token with the final committed index.
// BFT protocols guarantee safety internally (Algorithm 2 lines 5-6).
func (a *BFTAdapter) Checkpoint(ctx context.Context, _ *altunpb.Empty) (*altunpb.CheckpointToken, error) {
	idx := a.parseFinalIndex()
	h := sha256.Sum256([]byte(fmt.Sprintf("bft-%d-%d", a.id, idx)))
	return &altunpb.CheckpointToken{
		InstanceId: a.id,
		Index:      idx,
		StateHash:  h[:],
	}, nil
}

// Seal sends SIGTERM to the bft-server and waits for graceful shutdown.
func (a *BFTAdapter) Seal(ctx context.Context, req *altunpb.SealRequest) (*altunpb.SealResponse, error) {
	a.sealed.Store(true)
	if a.cmd == nil || a.cmd.Process == nil {
		return &altunpb.SealResponse{Error: "no subprocess running"}, nil
	}

	a.cmd.Process.Signal(syscall.SIGTERM)

	timeout := time.Duration(req.TimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	done := make(chan error, 1)
	go func() { done <- a.cmd.Wait() }()

	select {
	case <-time.After(timeout):
		a.cmd.Process.Kill()
		return &altunpb.SealResponse{
			FinalIndex: a.parseFinalIndex(),
			Error:      "drain timeout, force killed",
		}, nil
	case <-done:
		return &altunpb.SealResponse{FinalIndex: a.parseFinalIndex()}, nil
	}
}

// Restore restarts the subprocess (BFT has no external state to restore).
func (a *BFTAdapter) Restore(ctx context.Context, token *altunpb.CheckpointToken) (*altunpb.RestoreResponse, error) {
	a.sealed.Store(false)
	if err := a.Start(); err != nil {
		return &altunpb.RestoreResponse{Error: err.Error()}, nil
	}
	return &altunpb.RestoreResponse{}, nil
}

// IsSealed returns whether Seal has been called.
func (a *BFTAdapter) IsSealed(ctx context.Context, _ *altunpb.Empty) (*altunpb.SealedStatus, error) {
	return &altunpb.SealedStatus{Sealed: a.sealed.Load()}, nil
}

// GetInfo returns BFT instance metadata.
func (a *BFTAdapter) GetInfo(ctx context.Context, _ *altunpb.Empty) (*altunpb.InstanceInfo, error) {
	return &altunpb.InstanceInfo{
		Id:       a.id,
		Mode:     "BFT",
		Protocol: a.config.Protocol,
	}, nil
}

// parseFinalIndex reads throughput from bft-performance.json and estimates
// total committed commands = throughput * duration.
func (a *BFTAdapter) parseFinalIndex() int64 {
	path := fmt.Sprintf("%s/bft-performance.json", a.config.OutputDir)
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	var metrics struct {
		Throughput float64 `json:"throughput"`
	}
	if err := json.Unmarshal(data, &metrics); err != nil {
		return 0
	}
	return int64(metrics.Throughput * float64(a.config.Duration))
}
