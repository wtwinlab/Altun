// Package altunsm is the replicated key-value state machine shared by every
// consensus instance (Paxos, HotStuff) behind the Altun proxy.  Every replica
// of every instance applies the same deterministic transition function to the
// commands its consensus protocol orders.  Besides client get/put it
// implements the policy-window boundary commands of the virtual log:
//
//	seal      - agreed by the OLD instance: stop executing client ops in the
//	            segment, fix its tail (last global position) and record the
//	            transition (window, old/new family, next segment id) in the
//	            replicated segment table.
//	segstart  - agreed by the NEW instance as its first entry: install the
//	            snapshot staged out of band only if its digest equals the
//	            digest carried by the entry (the sealed state's digest);
//	            otherwise the replica marks itself diverged and serves nothing.
//
// The segment table, the KV map and the per-client dedup table are part of
// the snapshot, so they travel across every boundary and the full
// segment history can be replayed from any active instance.
package altunsm

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sync"
)

// Op is a command ordered by a consensus instance.
type Op struct {
	T      string       `json:"t"`              // get | put | seal | segstart | noop
	K      string       `json:"k,omitempty"`    // key
	V      string       `json:"v,omitempty"`    // value (put)
	C      uint64       `json:"c,omitempty"`    // client id
	S      uint64       `json:"s,omitempty"`    // client sequence number (one outstanding op per client)
	Seg    uint64       `json:"seg,omitempty"`  // seal: segment to seal; segstart: segment to start
	Digest string       `json:"d,omitempty"`    // segstart: digest the staged snapshot must have
	Rec    *Record      `json:"rec,omitempty"`  // transition record carried by seal / segstart
	Cert   *Certificate `json:"cert,omitempty"` // segstart (activate): the checkpoint certificate of the sealed state
}

// Record is a transition record of the virtual log, stored in the replicated
// segment table.
type Record struct {
	Kind   string `json:"kind"`              // seal | activate | abort
	Seg    uint64 `json:"seg"`               // seal: sealed seg; activate: started seg
	NewSeg uint64 `json:"new_seg,omitempty"` // seal: the segment registered next
	Window uint64 `json:"window"`            // policy-window id of the directive
	From   string `json:"from,omitempty"`    // family/protocol of the sealed segment
	To     string `json:"to,omitempty"`      // family/protocol of the next segment
	F      int    `json:"f,omitempty"`       // fault threshold of the next segment
	N      int    `json:"n,omitempty"`       // membership size of the next segment
	Tail   uint64 `json:"tail"`              // last global position of the sealed segment
	Start  uint64 `json:"start,omitempty"`   // first global position of the new segment
	Digest string `json:"digest,omitempty"`  // digest of the state at the boundary
	FB     int    `json:"fb,omitempty"`      // certification fault bound of the window
	Cert   string `json:"cert,omitempty"`    // activate: hash of the checkpoint certificate that was installed
	Ts     int64  `json:"ts,omitempty"`      // proxy wall clock when issued (informational)
}

// Result is the reply of a replica for one op.
type Result struct {
	OK     bool   `json:"ok"`
	Err    string `json:"err,omitempty"`
	V      string `json:"v,omitempty"`
	Found  bool   `json:"found,omitempty"`
	Prev   string `json:"p,omitempty"`   // put: the value it overwrote (a put is a swap)
	Pos    uint64 `json:"pos,omitempty"` // global virtual-log position of a client op
	Seg    uint64 `json:"seg"`           // segment the replica is in
	Tail   uint64 `json:"tail,omitempty"`
	Digest string `json:"d,omitempty"`
	Dup    bool   `json:"dup,omitempty"` // client op answered from the dedup table
}

// DedupEntry remembers the last executed op of a client.
type DedupEntry struct {
	S uint64 `json:"s"`
	R Result `json:"r"`
}

// Snapshot is the transferable state at a boundary.
type Snapshot struct {
	Seg   uint64                `json:"seg"`  // the sealed segment
	Tail  uint64                `json:"tail"` // last global position of the sealed segment
	KV    map[string]string     `json:"kv"`
	Dedup map[uint64]DedupEntry `json:"dedup"`
	Table []Record              `json:"table"`
}

// Digest is the SHA-256 of the canonical JSON encoding (encoding/json sorts map keys).
func (s *Snapshot) Digest() string {
	b, _ := json.Marshal(s)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// States of a replica's state machine.
const (
	Fresh    = "fresh"    // never activated
	Staged   = "staged"   // snapshot imported, waiting for its segstart entry
	Active   = "active"   // executing client ops of segment Seg
	Sealed   = "sealed"   // segment Seg sealed at Tail
	Diverged = "diverged" // segstart digest did not match the staged state
)

// Byzantine behaviours of a replica, selectable for fault-injection experiments.
const (
	ByzNone    = ""
	ByzCorrupt = "corrupt" // corrupts the snapshot it imports (its state digest differs); replies honestly
	ByzLie     = "lie"     // additionally lies: wrong digests in replies and serves a corrupted snapshot
	ByzLieOK   = "lie-ok"  // corrupts imported state but claims success with the expected digest
	ByzFork    = "fork"    // at the seal, consistently presents an older prefix of its own log: reports its tail and
	//                        digest, serves it as the sealed snapshot and signs an attestation for it
)

// forkEvery is how often (in executed client ops) a ByzFork replica refreshes
// the stale prefix it will present at the next seal.
const forkEvery = 500

// SM is the replicated state machine of one replica.
type SM struct {
	mu             sync.Mutex
	state          string
	seg            uint64
	pos            uint64 // last global position executed (or tail when sealed)
	start          uint64 // first global position of the current segment
	kv             map[string]string
	dedup          map[uint64]DedupEntry
	table          []Record
	staged         *Snapshot
	stagedSeg      uint64
	stagedDigest   string
	lastSeal       *Snapshot // state at the seal (served to the proxy)
	lastSealDigest string
	forkBase       *Snapshot // ByzFork: the stale prefix refreshed every forkEvery ops
	forkSeal       *Snapshot // ByzFork: the forged sealed state presented at the last seal
	forkDigest     string

	// certification
	Name     string
	priv     ed25519.PrivateKey
	pubs     map[string]ed25519.PublicKey
	attCache map[string]Attestation
	// RequireCert makes segstart(activate) refuse entries without a valid certificate.
	RequireCert bool

	Byz string
	// NoDigestCheck disables the segstart digest comparison.  It exists for
	// fault-injection experiments that measure what the check prevents.
	NoDigestCheck bool
	execCount     uint64
}

func New() *SM {
	return &SM{state: Fresh, kv: map[string]string{}, dedup: map[uint64]DedupEntry{}}
}

func enc(r Result) []byte { b, _ := json.Marshal(r); return b }

func (m *SM) snapshotLocked() *Snapshot {
	kv := make(map[string]string, len(m.kv))
	for k, v := range m.kv {
		kv[k] = v
	}
	dd := make(map[uint64]DedupEntry, len(m.dedup))
	for k, v := range m.dedup {
		dd[k] = v
	}
	tb := append([]Record(nil), m.table...)
	return &Snapshot{Seg: m.seg, Tail: m.pos, KV: kv, Dedup: dd, Table: tb}
}

// Apply executes one ordered command and returns the encoded Result.  It must
// be called by the consensus executor in log order.
func (m *SM) Apply(data []byte) []byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.execCount++
	if len(data) == 0 {
		return enc(Result{OK: true, Seg: m.seg})
	}
	var op Op
	if err := json.Unmarshal(data, &op); err != nil {
		return enc(Result{Err: "bad op", Seg: m.seg})
	}
	switch op.T {
	case "noop":
		return enc(Result{OK: true, Seg: m.seg})
	case "get", "put":
		if m.state != Active {
			return enc(Result{Err: "not-active:" + m.state, Seg: m.seg})
		}
		if d, ok := m.dedup[op.C]; ok && op.C != 0 {
			if op.S == d.S {
				r := d.R
				r.Dup = true
				return enc(r)
			}
			if op.S < d.S {
				return enc(Result{Err: "stale", Seg: m.seg})
			}
		}
		m.pos++
		r := Result{OK: true, Pos: m.pos, Seg: m.seg}
		if op.T == "put" {
			r.Prev = m.kv[op.K]
			m.kv[op.K] = op.V
		} else {
			r.V, r.Found = m.kv[op.K]
		}
		if op.C != 0 {
			m.dedup[op.C] = DedupEntry{S: op.S, R: r}
		}
		if m.Byz == ByzFork && (m.forkBase == nil || m.pos%forkEvery == 0) {
			m.forkBase = m.snapshotLocked()
		}
		return enc(r)
	case "seal":
		if m.state == Sealed && m.seg == op.Seg {
			return enc(m.sealReply())
		}
		if m.state != Active || m.seg != op.Seg {
			return enc(Result{Err: "cannot-seal:" + m.state, Seg: m.seg})
		}
		rec := Record{Kind: "seal", Seg: m.seg}
		if op.Rec != nil {
			rec = *op.Rec
			rec.Kind = "seal"
			rec.Seg = m.seg
		}
		rec.Tail = m.pos
		rec.Digest = "" // the digest covers the table including this record
		m.table = append(m.table, rec)
		m.state = Sealed
		m.lastSeal = m.snapshotLocked()
		m.lastSealDigest = m.lastSeal.Digest()
		if m.Byz == ByzFork {
			// the forged checkpoint: an older prefix of this replica's own log,
			// presented consistently (tail, digest, snapshot, attestation)
			base := m.forkBase
			if base == nil {
				base = m.lastSeal
			}
			fs := *base
			fs.Seg = m.seg
			fs.Table = append([]Record(nil), m.table...)
			fs.Table[len(fs.Table)-1].Tail = fs.Tail
			m.forkSeal = &fs
			m.forkDigest = fs.Digest()
		}
		return enc(m.sealReply())
	case "segstart":
		return enc(m.segstart(op))
	case "peek":
		// Captures the state for transfer without sealing; the segment keeps
		// executing client ops.  Used only by the proxy's -unsafe-noseal mode,
		// which exists to demonstrate the consequence of skipping the seal.
		if m.state != Active {
			return enc(Result{Err: "cannot-peek:" + m.state, Seg: m.seg})
		}
		m.lastSeal = m.snapshotLocked()
		m.lastSealDigest = m.lastSeal.Digest()
		return enc(Result{OK: true, Seg: m.seg, Tail: m.pos, Digest: m.lastSealDigest})
	}
	return enc(Result{Err: "unknown op", Seg: m.seg})
}

func (m *SM) sealReply() Result {
	r := Result{OK: true, Seg: m.seg, Tail: m.lastSeal.Tail, Digest: m.lastSealDigest}
	if m.Byz == ByzLie {
		r.Digest = "bad" + r.Digest[3:]
	}
	if m.Byz == ByzFork && m.forkSeal != nil {
		r.Tail, r.Digest = m.forkSeal.Tail, m.forkDigest
	}
	return r
}

func (m *SM) segstart(op Op) Result {
	if m.state == Active && m.seg == op.Seg {
		return Result{OK: true, Seg: m.seg, Digest: op.Digest, Tail: m.start - 1} // replayed entry
	}
	rec := Record{Kind: "activate", Seg: op.Seg}
	if op.Rec != nil {
		rec = *op.Rec
		rec.Seg = op.Seg
	}
	switch {
	case m.state == Staged && m.stagedSeg == op.Seg:
		if (m.RequireCert && op.Seg > 1 || op.Cert != nil) && !m.NoDigestCheck {
			// Checkpoint certification: install only a certified checkpoint
			// (segment 1 is the bootstrap of a fresh deployment on the empty
			// state).  The chain is carried by the snapshot's own table: the
			// certificate must extend the one installed with the sealed segment.
			if err := op.Cert.Verify(m.pubs, installedCertOf(m.staged.Table)); err != nil {
				return Result{Err: "bad-cert: " + err.Error(), Seg: m.seg}
			}
			if op.Cert.Digest != op.Digest || op.Cert.Seg+1 != op.Seg {
				return Result{Err: "bad-cert: certificate does not cover this segstart", Seg: m.seg}
			}
			rec.Cert = op.Cert.Hash()
		}
		if m.stagedDigest != op.Digest && !m.NoDigestCheck {
			m.state = Diverged
			d := m.stagedDigest
			if m.Byz == ByzLieOK {
				return Result{OK: true, Seg: op.Seg, Digest: op.Digest}
			}
			return Result{Err: "digest-mismatch", Seg: m.seg, Digest: d}
		}
		s := m.staged
		m.kv, m.dedup, m.table = s.KV, s.Dedup, s.Table
		if m.kv == nil {
			m.kv = map[string]string{}
		}
		if m.dedup == nil {
			m.dedup = map[uint64]DedupEntry{}
		}
		m.pos = s.Tail
		m.staged = nil
	case m.state == Sealed && op.Rec != nil && op.Rec.Kind == "abort" && m.lastSealDigest == op.Digest:
		// abort: the old instance resumes as a new segment on its unchanged sealed state
	default:
		return Result{Err: "cannot-start:" + m.state, Seg: m.seg}
	}
	rec.Tail = m.pos
	rec.Start = m.pos + 1
	rec.Digest = op.Digest
	m.table = append(m.table, rec)
	m.seg = op.Seg
	m.start = m.pos + 1
	m.state = Active
	r := Result{OK: true, Seg: m.seg, Digest: op.Digest, Tail: m.pos}
	if m.Byz == ByzLie {
		r.Digest = "bad" + r.Digest[3:]
	}
	return r
}

// Import stages a snapshot for segment newSeg (out-of-band state transfer).
// The staged state is installed only by an agreed segstart entry.
func (m *SM) Import(s *Snapshot, newSeg uint64) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state == Active && m.seg >= newSeg {
		return "", errors.New("instance active in segment >= target")
	}
	if m.state == Sealed && m.seg >= newSeg {
		return "", errors.New("instance sealed at segment >= target")
	}
	if m.Byz == ByzCorrupt || m.Byz == ByzLieOK {
		c := *s
		c.KV = map[string]string{}
		for k, v := range s.KV {
			c.KV[k] = v + "#byz"
		}
		c.KV["byz-marker"] = "x"
		s = &c
	}
	m.staged = s
	m.stagedSeg = newSeg
	m.stagedDigest = s.Digest()
	m.state = Staged
	return m.stagedDigest, nil
}

// SealedSnapshot returns the state at the last seal of this replica.
func (m *SM) SealedSnapshot() (*Snapshot, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.lastSeal == nil {
		return nil, "", errors.New("never sealed")
	}
	if m.Byz == ByzLie {
		c := *m.lastSeal
		c.KV = map[string]string{"byz-marker": "x"}
		return &c, m.lastSealDigest, nil
	}
	if m.Byz == ByzFork && m.forkSeal != nil {
		return m.forkSeal, m.forkDigest, nil
	}
	return m.lastSeal, m.lastSealDigest, nil
}

// Status is the externally visible replica status.
type Status struct {
	State      string   `json:"state"`
	Seg        uint64   `json:"seg"`
	Pos        uint64   `json:"pos"`
	Start      uint64   `json:"start"`
	Table      []Record `json:"table"`
	SealDigest string   `json:"seal_digest,omitempty"`
	StagedSeg  uint64   `json:"staged_seg,omitempty"`
	Exec       uint64   `json:"exec"`
	Keys       int      `json:"keys"`
}

func (m *SM) Status() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	return Status{State: m.state, Seg: m.seg, Pos: m.pos, Start: m.start,
		Table: append([]Record(nil), m.table...), SealDigest: m.lastSealDigest,
		StagedSeg: m.stagedSeg, Exec: m.execCount, Keys: len(m.kv)}
}
