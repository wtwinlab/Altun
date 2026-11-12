// Package adapter provides a gRPC ConsensusInstance server that wraps an
// in-process CFT protocol replica (paxos, epaxos, vpaxos, wpaxos).
package adapter

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/wtwinlab/altun/consensus/cft"
	"github.com/wtwinlab/altun/proto/altunpb"
)

// CommitTracker abstracts protocol-specific commit status queries.
// Each CFT protocol implements this to expose its commit state.
type CommitTracker interface {
	// LastCommittedSlot returns the highest slot that has been executed.
	LastCommittedSlot() int
	// AllSlotsCommitted returns true if no in-flight uncommitted slots exist.
	AllSlotsCommitted() bool
}

// CFTAdapter implements the gRPC ConsensusInstance service for CFT protocols.
type CFTAdapter struct {
	altunpb.UnimplementedConsensusInstanceServer

	id       int64
	protocol string
	node     cft.Node
	tracker  CommitTracker
	sealed   atomic.Bool
}

// NewCFTAdapter wraps any CFT replica (paxos/epaxos/vpaxos/wpaxos) behind
// the gRPC interface. The CommitTracker provides protocol-specific commit
// queries; Node provides Propose/Snapshot/Restore via the common interface.
func NewCFTAdapter(id int64, protocol string, node cft.Node, tracker CommitTracker) *CFTAdapter {
	return &CFTAdapter{
		id:       id,
		protocol: protocol,
		node:     node,
		tracker:  tracker,
	}
}

// Propose injects a KV command via Node.InjectRequest and blocks until reply.
func (a *CFTAdapter) Propose(ctx context.Context, req *altunpb.ProposeRequest) (*altunpb.ProposeResponse, error) {
	if a.sealed.Load() {
		return &altunpb.ProposeResponse{Error: "instance sealed"}, nil
	}

	cmd := cft.Command{
		Key:       cft.Key(req.Key),
		Value:     req.Value,
		ClientID:  cft.ID(req.ClientId),
		CommandID: int(req.CommandId),
	}
	r := cft.Request{Command: cmd, Timestamp: time.Now().UnixNano()}
	replyCh := a.node.InjectRequest(r)

	select {
	case <-ctx.Done():
		return &altunpb.ProposeResponse{Error: ctx.Err().Error()}, nil
	case reply := <-replyCh:
		if reply.Err != nil {
			return &altunpb.ProposeResponse{Error: reply.Err.Error()}, nil
		}
		return &altunpb.ProposeResponse{Value: reply.Value}, nil
	}
}

// LastCommitted returns the highest executed slot and a state hash.
func (a *CFTAdapter) LastCommitted(ctx context.Context, _ *altunpb.Empty) (*altunpb.CommitStatus, error) {
	idx := int64(a.tracker.LastCommittedSlot())
	h := a.stateHash()
	return &altunpb.CommitStatus{Index: idx, StateHash: h}, nil
}

// Checkpoint serializes the full KV state into a CheckpointToken.
func (a *CFTAdapter) Checkpoint(ctx context.Context, _ *altunpb.Empty) (*altunpb.CheckpointToken, error) {
	snap := a.node.Snapshot()
	state, _ := json.Marshal(snap)
	h := sha256.Sum256(state)
	return &altunpb.CheckpointToken{
		InstanceId: a.id,
		Index:      int64(a.tracker.LastCommittedSlot()),
		StateHash:  h[:],
		State:      state,
	}, nil
}

// Seal marks the instance as sealed and waits for in-flight slots to commit.
func (a *CFTAdapter) Seal(ctx context.Context, req *altunpb.SealRequest) (*altunpb.SealResponse, error) {
	a.sealed.Store(true)

	timeout := time.Duration(req.TimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	deadline := time.After(timeout)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-deadline:
			return &altunpb.SealResponse{
				FinalIndex: int64(a.tracker.LastCommittedSlot()),
				Error:      "drain timeout",
			}, nil
		case <-ticker.C:
			if a.tracker.AllSlotsCommitted() {
				return &altunpb.SealResponse{
					FinalIndex: int64(a.tracker.LastCommittedSlot()),
				}, nil
			}
		}
	}
}

// Restore replaces the KV state from a checkpoint token.
func (a *CFTAdapter) Restore(ctx context.Context, token *altunpb.CheckpointToken) (*altunpb.RestoreResponse, error) {
	var snap map[cft.Key]cft.Value
	if err := json.Unmarshal(token.State, &snap); err != nil {
		return &altunpb.RestoreResponse{Error: fmt.Sprintf("unmarshal: %v", err)}, nil
	}
	a.node.LoadSnapshot(snap)
	return &altunpb.RestoreResponse{}, nil
}

// IsSealed returns the current seal status.
func (a *CFTAdapter) IsSealed(ctx context.Context, _ *altunpb.Empty) (*altunpb.SealedStatus, error) {
	return &altunpb.SealedStatus{Sealed: a.sealed.Load()}, nil
}

// GetInfo returns instance metadata.
func (a *CFTAdapter) GetInfo(ctx context.Context, _ *altunpb.Empty) (*altunpb.InstanceInfo, error) {
	return &altunpb.InstanceInfo{
		Id:       a.id,
		Mode:     "CFT",
		Protocol: a.protocol,
	}, nil
}

func (a *CFTAdapter) stateHash() []byte {
	snap := a.node.Snapshot()
	data, _ := json.Marshal(snap)
	h := sha256.Sum256(data)
	return h[:]
}
