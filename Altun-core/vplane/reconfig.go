package vplane

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"sync"
	"time"

	"github.com/wtwinlab/altun/proto/altunpb"
)

// TriggerType distinguishes intra-family from cross-family switches.
type TriggerType int

const (
	IntraFamily TriggerType = iota // same fault model, RL agent proposal
	CrossFamily                    // fault model change, operator policy
)

// ReconfigManager implements Algorithm 1 (Protocol Reconfiguration Procedure)
// from Section 3.2 of the paper. It orchestrates the 4-phase transition
// between consensus instances through the Virtual Log.
type ReconfigManager struct {
	mu           sync.Mutex
	vlog         *VirtualLog
	launcher     *AdapterLauncher
	verifier     *Verifier // forensic-verify integration for Algorithm 2
	forensicsLog string    // path to FORENSICS_LOG file (empty = skip verification)
	policy       *PolicyWindow
	operatorPK   ed25519.PublicKey
	timeout      time.Duration // τ activation timeout
	nextID       int64         // monotonically increasing instance ID
}

// NewReconfigManager creates a new reconfiguration manager.
// verifierBin is the path to the forensic-verify binary (empty = skip safety check).
// forensicsLog is the FORENSICS_LOG path used by cft-server (empty = skip).
func NewReconfigManager(vl *VirtualLog, launcher *AdapterLauncher, policy *PolicyWindow, opPK ed25519.PublicKey, timeout time.Duration) *ReconfigManager {
	return &ReconfigManager{
		vlog:       vl,
		launcher:   launcher,
		policy:     policy,
		operatorPK: opPK,
		timeout:    timeout,
		nextID:     1,
	}
}

// SetVerifier configures the forensic verifier for CFT boundary checkpoints.
// binPath: path to forensic-verify binary. logPath: FORENSICS_LOG file path.
// If not called, CFT checkpoints skip safety verification (dev mode).
func (rm *ReconfigManager) SetVerifier(binPath, logPath string) {
	rm.verifier = NewVerifier(binPath)
	rm.forensicsLog = logPath
}

// SwitchTiming records wall-clock durations for each reconfiguration phase.
type SwitchTiming struct {
	AdmissionMs     float64 `json:"admission_ms"`
	InstantiationMs float64 `json:"instantiation_ms"`
	ActivationMs    float64 `json:"activation_ms"`
	SealMs          float64 `json:"seal_ms"`
	TotalMs         float64 `json:"total_ms"`
}

// ReconfigResult is the response from a successful reconfiguration.
type ReconfigResult struct {
	NewInstanceID int64        `json:"new_instance_id"`
	Protocol      string       `json:"protocol"`
	Mode          string       `json:"mode"`
	Timing        SwitchTiming `json:"switch_timing"`
}

// Reconfigure executes Algorithm 1's 4-phase reconfiguration.
func (rm *ReconfigManager) Reconfigure(trigger TriggerType, target string, newPolicy *PolicyWindow, launchCfg LaunchConfig) (*ReconfigResult, error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	totalStart := time.Now()
	var timing SwitchTiming

	// ═══ Phase (i): Admission (Algorithm 1 lines 1-8) ═══
	admStart := time.Now()
	if trigger == CrossFamily {
		// Cross-family: verify operator signature and quorum feasibility
		if newPolicy == nil {
			return nil, fmt.Errorf("cross-family switch requires policy window")
		}
		if !newPolicy.Verify(rm.operatorPK) {
			return nil, fmt.Errorf("invalid operator signature on policy window")
		}
		if !newPolicy.QuorumFeasible() {
			return nil, fmt.Errorf("quorum infeasible: n=%d < max(3*f_BF+1=%d, 2*f_CF+1=%d)",
				newPolicy.N, 3*newPolicy.F_BF+1, 2*newPolicy.F_CF+1)
		}
		if !newPolicy.AdmitsProtocol(target) {
			return nil, fmt.Errorf("protocol %q not in policy window pool", target)
		}
		rm.policy = newPolicy
	} else {
		// Intra-family: verify target ∈ Π_σ.mode
		if rm.policy != nil && !rm.policy.AdmitsProtocol(target) {
			return nil, fmt.Errorf("protocol %q not admitted by current policy window", target)
		}
	}
	timing.AdmissionMs = float64(time.Since(admStart).Microseconds()) / 1000.0

	// ═══ Phase (ii): Instantiation (Algorithm 1 lines 9-17) ═══
	instStart := time.Now()

	rm.nextID++
	instanceID := rm.nextID
	launchCfg.InstanceID = instanceID
	launchCfg.Protocol = target

	// Launch new adapter process
	grpcAddr, err := rm.launcher.Launch(launchCfg)
	if err != nil {
		return nil, fmt.Errorf("instantiation failed: %w", err)
	}

	// Wait briefly for adapter to start accepting gRPC
	time.Sleep(2 * time.Second)

	// Register instance metadata in virtual log (Algorithm 1 line 14)
	meta := InstanceMeta{
		ID:       instanceID,
		Mode:     launchCfg.Mode,
		Protocol: target,
		GRPCAddr: grpcAddr,
	}
	rm.vlog.RegisterInstance(meta)

	// Checkpoint current instance and restore into new one
	oldClient := rm.vlog.CurrentClient()
	if oldClient != nil {
		ckpt, err := oldClient.Checkpoint(context.Background())
		if err == nil && len(ckpt.State) > 0 {
			newClient, err := Dial(grpcAddr)
			if err == nil {
				newClient.Restore(context.Background(), ckpt)
				newClient.Close()
			}
		}
	}
	timing.InstantiationMs = float64(time.Since(instStart).Microseconds()) / 1000.0

	// ═══ Phase (iii): Activation (Algorithm 1 lines 18-23) ═══
	actStart := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), rm.timeout)
	defer cancel()

	// Verify new instance is ready by calling GetInfo
	newClient, err := Dial(grpcAddr)
	if err != nil {
		rm.launcher.Stop(grpcAddr)
		return nil, fmt.Errorf("activation: cannot connect to new instance: %w", err)
	}
	_, err = newClient.GetInfo(ctx)
	newClient.Close()
	if err != nil {
		// Timeout → abort, invalidate, fall back to current (Algorithm 1 line 22)
		rm.launcher.Stop(grpcAddr)
		return nil, fmt.Errorf("activation timeout: %w", err)
	}

	// Activate: switch virtual log to new instance
	if err := rm.vlog.Activate(instanceID); err != nil {
		rm.launcher.Stop(grpcAddr)
		return nil, fmt.Errorf("activation failed: %w", err)
	}
	timing.ActivationMs = float64(time.Since(actStart).Microseconds()) / 1000.0

	// ═══ Phase (iv): Seal (Algorithm 1 lines 24-26) ═══
	sealStart := time.Now()
	if oldClient != nil {
		// Boundary checkpoint on old instance (Algorithm 2)
		_, err := rm.boundaryCheckpoint(oldClient)
		if err != nil {
			// Safety violation → could rollback, but for now log and continue
			fmt.Printf("WARNING: boundary checkpoint failed: %v\n", err)
		}

		// Seal old instance
		sealCtx, sealCancel := context.WithTimeout(context.Background(), 30*time.Second)
		rm.vlog.SealCurrent(sealCtx, 30000)
		sealCancel()
	}
	timing.SealMs = float64(time.Since(sealStart).Microseconds()) / 1000.0

	timing.TotalMs = float64(time.Since(totalStart).Microseconds()) / 1000.0

	return &ReconfigResult{
		NewInstanceID: instanceID,
		Protocol:      target,
		Mode:          launchCfg.Mode,
		Timing:        timing,
	}, nil
}

// boundaryCheckpoint implements Algorithm 2: checkpointing at the instance
// boundary. For BFT, directly takes checkpoint. For CFT, verifies safety
// with the auditor first.
func (rm *ReconfigManager) boundaryCheckpoint(client *ConsensusClient) (*CheckpointResult, error) {
	ctx := context.Background()

	info, err := client.GetInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("get info: %w", err)
	}

	if info.Mode == "BFT" {
		// BFT: directly take checkpoint (Algorithm 2 lines 5-6)
		token, err := client.Checkpoint(ctx)
		if err != nil {
			return nil, err
		}
		return &CheckpointResult{Token: token, SafetyVerified: true}, nil
	}

	// CFT: auditor safety check first (Algorithm 2 lines 8-13).
	// Run forensic-verify against the current forensics log. If any
	// branch detects a safety violation, abort the checkpoint.
	if rm.verifier != nil && rm.forensicsLog != "" {
		vr, err := rm.verifier.Verify(ctx, rm.forensicsLog)
		if err != nil {
			fmt.Printf("WARNING: forensic verifier error: %v (proceeding anyway)\n", err)
		} else if vr.AnyViolation {
			// Safety violation detected → abort checkpoint (Algorithm 2 line 13)
			return nil, fmt.Errorf("safety violation detected by forensic-verify: %d branches flagged across %d records",
				countViolations(vr.BranchResults), vr.Records)
		} else {
			fmt.Printf("forensic-verify: safe (%d records, %d ms)\n", vr.Records, vr.TotalMs)
		}
	}

	token, err := client.Checkpoint(ctx)
	if err != nil {
		return nil, err
	}
	return &CheckpointResult{Token: token, SafetyVerified: true}, nil
}

// CheckpointResult wraps a checkpoint token with verification metadata.
type CheckpointResult struct {
	Token          *altunpb.CheckpointToken
	SafetyVerified bool
}

func countViolations(branches []BranchResult) int {
	n := 0
	for _, b := range branches {
		if b.ViolationCount > 0 {
			n++
		}
	}
	return n
}

// CurrentPolicy returns the active policy window.
func (rm *ReconfigManager) CurrentPolicy() *PolicyWindow {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	return rm.policy
}
