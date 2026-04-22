package vplane

import (
	"context"
	"fmt"
	"sync"

	"github.com/wtwinlab/altun/proto/altunpb"
)

// InstanceMeta stores metadata for one consensus instance in the virtual log.
type InstanceMeta struct {
	ID        int64  `json:"id"`
	Mode      string `json:"mode"`     // "CFT" or "BFT"
	Protocol  string `json:"protocol"` // "paxos", "chainedhotstuff", etc.
	GRPCAddr  string `json:"grpc_addr"`
	StartAddr int64  `json:"start_addr"` // first virtual address
	EndAddr   int64  `json:"end_addr"`   // filled after Seal (-1 = active)
	Sealed    bool   `json:"sealed"`
}

// VirtualLog manages a sequence of consensus instances and provides a
// globally ordered log abstraction across reconfigurations (Section 3.1).
// Persistence is delegated to a pluggable Store (JSON or Ledger).
type VirtualLog struct {
	mu        sync.RWMutex
	instances []*InstanceMeta
	current   *ConsensusClient // gRPC client to active instance
	globalIdx int64            // next virtual address to assign
	store     Store            // pluggable persistence backend
}

// New creates a VirtualLog with the given persistence store.
// Use NewJSONStore("path.json") for file-based or NewLedgerStore("dir", false) for Badger.
func New(store Store) *VirtualLog {
	vl := &VirtualLog{
		instances: make([]*InstanceMeta, 0),
		globalIdx: 0,
		store:     store,
	}
	vl.loadMeta()
	return vl
}

// NewWithJSONFile is a convenience constructor that uses JSONStore (default).
func NewWithJSONFile(metaPath string) *VirtualLog {
	return New(NewJSONStore(metaPath))
}

// RegisterInstance adds a new instance's metadata to the log.
// Corresponds to Algorithm 1 line 14: register metadata M_{i+1} to L.
func (vl *VirtualLog) RegisterInstance(meta InstanceMeta) error {
	vl.mu.Lock()
	defer vl.mu.Unlock()

	meta.StartAddr = vl.globalIdx
	meta.EndAddr = -1
	meta.Sealed = false
	vl.instances = append(vl.instances, &meta)
	vl.persist()
	return nil
}

// Activate connects to a registered instance and makes it the active one.
// Corresponds to Algorithm 1 lines 20-21.
func (vl *VirtualLog) Activate(instanceID int64) error {
	vl.mu.Lock()
	defer vl.mu.Unlock()

	meta := vl.findInstanceLocked(instanceID)
	if meta == nil {
		return fmt.Errorf("instance %d not found", instanceID)
	}

	client, err := Dial(meta.GRPCAddr)
	if err != nil {
		return fmt.Errorf("connect to %s: %w", meta.GRPCAddr, err)
	}

	if vl.current != nil {
		vl.current.Close()
	}
	vl.current = client
	return nil
}

// Append proposes a command through the current active instance.
func (vl *VirtualLog) Append(ctx context.Context, key int32, value []byte) (int64, []byte, error) {
	vl.mu.RLock()
	c := vl.current
	vl.mu.RUnlock()

	if c == nil {
		return 0, nil, fmt.Errorf("no active instance")
	}

	resp, err := c.Propose(ctx, key, value, "vlog", 0)
	if err != nil {
		return 0, nil, err
	}
	if resp.Error != "" {
		return 0, nil, fmt.Errorf(resp.Error)
	}

	vl.mu.Lock()
	addr := vl.globalIdx
	vl.globalIdx++
	vl.mu.Unlock()

	return addr, resp.Value, nil
}

// SealCurrent seals the active instance and records the final address.
// Corresponds to Algorithm 1 line 26.
func (vl *VirtualLog) SealCurrent(ctx context.Context, timeoutMs int64) error {
	vl.mu.Lock()
	defer vl.mu.Unlock()

	if vl.current == nil {
		return fmt.Errorf("no active instance")
	}

	resp, err := vl.current.Seal(ctx, timeoutMs)
	if err != nil {
		return err
	}
	if resp.Error != "" {
		return fmt.Errorf("seal: %s", resp.Error)
	}

	for i := len(vl.instances) - 1; i >= 0; i-- {
		if !vl.instances[i].Sealed {
			vl.instances[i].Sealed = true
			vl.instances[i].EndAddr = vl.globalIdx
			break
		}
	}
	vl.persist()
	return nil
}

// SaveCheckpoint persists a checkpoint token for the given instance.
func (vl *VirtualLog) SaveCheckpoint(instanceID int64, token *altunpb.CheckpointToken) error {
	return vl.store.SaveCheckpoint(instanceID, token)
}

// LoadCheckpoint retrieves a previously saved checkpoint token.
func (vl *VirtualLog) LoadCheckpoint(instanceID int64) (*altunpb.CheckpointToken, error) {
	return vl.store.LoadCheckpoint(instanceID)
}

// CurrentInfo returns metadata of the current active instance.
func (vl *VirtualLog) CurrentInfo(ctx context.Context) (*altunpb.InstanceInfo, error) {
	vl.mu.RLock()
	c := vl.current
	vl.mu.RUnlock()
	if c == nil {
		return nil, fmt.Errorf("no active instance")
	}
	return c.GetInfo(ctx)
}

// CurrentClient returns the gRPC client for the active instance.
func (vl *VirtualLog) CurrentClient() *ConsensusClient {
	vl.mu.RLock()
	defer vl.mu.RUnlock()
	return vl.current
}

// Instances returns a copy of all instance metadata.
func (vl *VirtualLog) Instances() []InstanceMeta {
	vl.mu.RLock()
	defer vl.mu.RUnlock()
	out := make([]InstanceMeta, len(vl.instances))
	for i, m := range vl.instances {
		out[i] = *m
	}
	return out
}

// Close releases the persistence store.
func (vl *VirtualLog) Close() error {
	return vl.store.Close()
}

func (vl *VirtualLog) findInstanceLocked(id int64) *InstanceMeta {
	for _, m := range vl.instances {
		if m.ID == id {
			return m
		}
	}
	return nil
}

// persist writes current state to the store. Called with mu held.
func (vl *VirtualLog) persist() {
	if vl.store != nil {
		vl.store.SaveMeta(vl.instances, vl.globalIdx)
	}
}

// loadMeta restores state from the store on startup.
func (vl *VirtualLog) loadMeta() {
	if vl.store == nil {
		return
	}
	instances, globalIdx, err := vl.store.LoadMeta()
	if err != nil || instances == nil {
		return
	}
	vl.instances = instances
	vl.globalIdx = globalIdx
}
