package vplane

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/wtwinlab/altun/proto/altunpb"
	"github.com/wtwinlab/altun/storage/ledger"
)

// Store abstracts persistence for VirtualLog metadata and checkpoint tokens.
// Two implementations: JSONStore (simple file, default) and LedgerStore (Badger KV).
type Store interface {
	// SaveMeta persists instance metadata list.
	SaveMeta(instances []*InstanceMeta, globalIdx int64) error
	// LoadMeta restores instance metadata and globalIdx.
	LoadMeta() ([]*InstanceMeta, int64, error)
	// SaveCheckpoint persists a checkpoint token for an instance.
	SaveCheckpoint(instanceID int64, token *altunpb.CheckpointToken) error
	// LoadCheckpoint retrieves the checkpoint token for an instance.
	LoadCheckpoint(instanceID int64) (*altunpb.CheckpointToken, error)
	// Close releases underlying resources.
	Close() error
}

// ──────────────────────────────────────────────
// JSONStore — simple file-based persistence (default)
// ──────────────────────────────────────────────

type jsonPersisted struct {
	Instances []*InstanceMeta `json:"instances"`
	GlobalIdx int64           `json:"global_idx"`
}

// JSONStore persists metadata to a JSON file and checkpoint tokens to
// separate files in the same directory. Suitable for development and
// experiments where Badger overhead is not needed.
type JSONStore struct {
	metaPath string // e.g. "/data-ssd/.../vplane_meta.json"
}

// NewJSONStore creates a JSON file store. Pass "" to disable persistence.
func NewJSONStore(metaPath string) *JSONStore {
	return &JSONStore{metaPath: metaPath}
}

func (s *JSONStore) SaveMeta(instances []*InstanceMeta, globalIdx int64) error {
	if s.metaPath == "" {
		return nil
	}
	p := jsonPersisted{Instances: instances, GlobalIdx: globalIdx}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.metaPath, data, 0644)
}

func (s *JSONStore) LoadMeta() ([]*InstanceMeta, int64, error) {
	if s.metaPath == "" {
		return nil, 0, nil
	}
	data, err := os.ReadFile(s.metaPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, 0, nil
		}
		return nil, 0, err
	}
	var p jsonPersisted
	if err := json.Unmarshal(data, &p); err != nil {
		// Backward compat: try old format (just array of instances)
		var instances []*InstanceMeta
		if err2 := json.Unmarshal(data, &instances); err2 == nil {
			var maxIdx int64
			for _, m := range instances {
				if m.EndAddr > maxIdx {
					maxIdx = m.EndAddr
				}
				if m.StartAddr > maxIdx {
					maxIdx = m.StartAddr
				}
			}
			return instances, maxIdx, nil
		}
		return nil, 0, err
	}
	return p.Instances, p.GlobalIdx, nil
}

func (s *JSONStore) SaveCheckpoint(instanceID int64, token *altunpb.CheckpointToken) error {
	if s.metaPath == "" {
		return nil
	}
	path := fmt.Sprintf("%s.ckpt.%d.json", s.metaPath, instanceID)
	data, err := json.Marshal(token)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func (s *JSONStore) LoadCheckpoint(instanceID int64) (*altunpb.CheckpointToken, error) {
	if s.metaPath == "" {
		return nil, fmt.Errorf("no persistence path")
	}
	path := fmt.Sprintf("%s.ckpt.%d.json", s.metaPath, instanceID)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var token altunpb.CheckpointToken
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, err
	}
	return &token, nil
}

func (s *JSONStore) Close() error { return nil }

// ──────────────────────────────────────────────
// LedgerStore — Badger KV + Sparse Merkle Tree
// ──────────────────────────────────────────────

var (
	ledgerMetaKey = []byte("vplane:meta")
	ledgerCkptPfx = []byte("vplane:ckpt:")
)

// LedgerStore uses the existing storage/ledger package (Badger + optional
// Merkle tree) for production-grade persistence with crash recovery and
// tamper-evident proofs.
type LedgerStore struct {
	lg *ledger.LogLedger
}

// NewLedgerStore opens a Badger-backed store at the given directory.
// Set withMerkle=true to enable Sparse Merkle Tree proofs on metadata.
func NewLedgerStore(dbPath string, withMerkle bool) (*LedgerStore, error) {
	lg, err := ledger.NewLedger(dbPath, withMerkle)
	if err != nil {
		return nil, fmt.Errorf("open ledger at %s: %w", dbPath, err)
	}
	return &LedgerStore{lg: lg}, nil
}

func (s *LedgerStore) SaveMeta(instances []*InstanceMeta, globalIdx int64) error {
	p := jsonPersisted{Instances: instances, GlobalIdx: globalIdx}
	data, err := json.Marshal(p)
	if err != nil {
		return err
	}
	return s.lg.Append(ledgerMetaKey, data)
}

func (s *LedgerStore) LoadMeta() ([]*InstanceMeta, int64, error) {
	// LogLedger doesn't expose a Get(); read via Badger directly.
	// For now, fall back to the Append-based approach: the latest write
	// to ledgerMetaKey is the current state.
	// This is a simplification — production would use versioned reads.
	return nil, 0, fmt.Errorf("LedgerStore.LoadMeta not yet implemented; use JSONStore for now")
}

func (s *LedgerStore) SaveCheckpoint(instanceID int64, token *altunpb.CheckpointToken) error {
	key := append(ledgerCkptPfx, []byte(fmt.Sprintf("%d", instanceID))...)
	data, err := json.Marshal(token)
	if err != nil {
		return err
	}
	return s.lg.Append(key, data)
}

func (s *LedgerStore) LoadCheckpoint(instanceID int64) (*altunpb.CheckpointToken, error) {
	return nil, fmt.Errorf("LedgerStore.LoadCheckpoint not yet implemented; use JSONStore for now")
}

func (s *LedgerStore) Close() error {
	return s.lg.Close()
}
