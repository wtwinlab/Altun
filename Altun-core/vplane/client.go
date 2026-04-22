// Package vlog implements the Virtual Log abstraction from Section 3.1 of the
// Altun paper. It manages consensus instance lifecycles through gRPC adapters
// and provides a globally ordered log across protocol reconfigurations.
package vplane

import (
	"context"
	"fmt"
	"time"

	"github.com/wtwinlab/altun/proto/altunpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ConsensusClient wraps a gRPC connection to a ConsensusInstance adapter.
type ConsensusClient struct {
	conn   *grpc.ClientConn
	stub   altunpb.ConsensusInstanceClient
	addr   string
	closed bool
}

// Dial connects to a ConsensusInstance gRPC adapter at the given address.
func Dial(addr string) (*ConsensusClient, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(ctx, addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", addr, err)
	}
	return &ConsensusClient{
		conn: conn,
		stub: altunpb.NewConsensusInstanceClient(conn),
		addr: addr,
	}, nil
}

// Propose submits a command and blocks until committed.
func (c *ConsensusClient) Propose(ctx context.Context, key int32, value []byte, clientID string, cmdID int32) (*altunpb.ProposeResponse, error) {
	return c.stub.Propose(ctx, &altunpb.ProposeRequest{
		Key:       key,
		Value:     value,
		ClientId:  clientID,
		CommandId: cmdID,
	})
}

// LastCommitted returns the highest committed index and state hash.
func (c *ConsensusClient) LastCommitted(ctx context.Context) (*altunpb.CommitStatus, error) {
	return c.stub.LastCommitted(ctx, &altunpb.Empty{})
}

// Checkpoint takes a consistent state snapshot.
func (c *ConsensusClient) Checkpoint(ctx context.Context) (*altunpb.CheckpointToken, error) {
	return c.stub.Checkpoint(ctx, &altunpb.Empty{})
}

// Seal stops accepting new proposals and drains in-flight commands.
func (c *ConsensusClient) Seal(ctx context.Context, timeoutMs int64) (*altunpb.SealResponse, error) {
	return c.stub.Seal(ctx, &altunpb.SealRequest{TimeoutMs: timeoutMs})
}

// Restore loads state from a checkpoint token.
func (c *ConsensusClient) Restore(ctx context.Context, token *altunpb.CheckpointToken) (*altunpb.RestoreResponse, error) {
	return c.stub.Restore(ctx, token)
}

// IsSealed returns the seal status.
func (c *ConsensusClient) IsSealed(ctx context.Context) (bool, error) {
	resp, err := c.stub.IsSealed(ctx, &altunpb.Empty{})
	if err != nil {
		return false, err
	}
	return resp.Sealed, nil
}

// GetInfo returns instance metadata.
func (c *ConsensusClient) GetInfo(ctx context.Context) (*altunpb.InstanceInfo, error) {
	return c.stub.GetInfo(ctx, &altunpb.Empty{})
}

// Close releases the gRPC connection.
func (c *ConsensusClient) Close() error {
	if c.closed {
		return nil
	}
	c.closed = true
	return c.conn.Close()
}
