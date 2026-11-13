package vplane

import (
	"fmt"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
)

// AdapterLauncher manages the lifecycle of gRPC adapter processes.
type AdapterLauncher struct {
	mu           sync.Mutex
	cftBin       string // path to cft-adapter binary
	bftBin       string // path to bft-adapter binary
	nextPort     atomic.Int32
	runningProcs map[string]*exec.Cmd // grpcAddr → process
}

// NewAdapterLauncher creates a launcher with paths to adapter binaries.
func NewAdapterLauncher(cftBin, bftBin string, basePort int) *AdapterLauncher {
	l := &AdapterLauncher{
		cftBin:       cftBin,
		bftBin:       bftBin,
		runningProcs: make(map[string]*exec.Cmd),
	}
	l.nextPort.Store(int32(basePort))
	return l
}

// LaunchConfig holds parameters for launching an adapter.
type LaunchConfig struct {
	Protocol   string
	Mode       string // "CFT" or "BFT"
	InstanceID int64
	ConfigPath string // path to cluster config.json (for CFT)
	// BFT-specific fields
	Replicas    int
	Clients     int
	BatchSize   int
	PayloadSize int
	RateLimit   int
	Duration    int
	OutputDir   string
	BFTBinPath  string // path to bft-server (for BFT adapter to spawn)
}

// Launch starts an adapter process and returns its gRPC address.
func (l *AdapterLauncher) Launch(cfg LaunchConfig) (string, error) {
	port := int(l.nextPort.Add(1))
	addr := fmt.Sprintf("localhost:%d", port)

	var cmd *exec.Cmd
	if cfg.Mode == "CFT" {
		cmd = exec.Command(l.cftBin,
			"--protocol", cfg.Protocol,
			"--grpc-port", fmt.Sprintf("%d", port),
			"--instance-id", fmt.Sprintf("%d", cfg.InstanceID),
		)
	} else {
		cmd = exec.Command(l.bftBin,
			"--protocol", cfg.Protocol,
			"--grpc-port", fmt.Sprintf("%d", port),
			"--instance-id", fmt.Sprintf("%d", cfg.InstanceID),
			"--bft-bin", cfg.BFTBinPath,
			"--replicas", fmt.Sprintf("%d", cfg.Replicas),
			"--clients", fmt.Sprintf("%d", cfg.Clients),
			"--batch-size", fmt.Sprintf("%d", cfg.BatchSize),
			"--payload-size", fmt.Sprintf("%d", cfg.PayloadSize),
			"--rate-limit", fmt.Sprintf("%d", cfg.RateLimit),
			"--duration", fmt.Sprintf("%d", cfg.Duration),
			"--output", cfg.OutputDir,
		)
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("start adapter: %w", err)
	}

	l.mu.Lock()
	l.runningProcs[addr] = cmd
	l.mu.Unlock()

	return addr, nil
}

// Stop terminates the adapter process at the given gRPC address.
func (l *AdapterLauncher) Stop(addr string) error {
	l.mu.Lock()
	cmd, ok := l.runningProcs[addr]
	if ok {
		delete(l.runningProcs, addr)
	}
	l.mu.Unlock()

	if !ok || cmd.Process == nil {
		return nil
	}
	cmd.Process.Kill()
	cmd.Wait()
	return nil
}

// StopAll terminates all running adapter processes.
func (l *AdapterLauncher) StopAll() {
	l.mu.Lock()
	defer l.mu.Unlock()
	for addr, cmd := range l.runningProcs {
		if cmd.Process != nil {
			cmd.Process.Kill()
			cmd.Wait()
		}
		delete(l.runningProcs, addr)
	}
}
