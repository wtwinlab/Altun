// hashchain.go implements per-node hash chains for O(n) forensic evidence
// storage (Section 4.2 of the paper).
//
// Each node maintains a chain: H_i = SHA-256(H_{i-1} || entry_i).
// The auditor only needs to verify the chain tip plus the most recent u
// uncommitted entries, reducing storage from O(n*m) to O(n*u) where u is
// the in-flight concurrency window.
package auditor

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
)

// HashChain maintains a hash chain for one replica's forensic evidence.
type HashChain struct {
	mu      sync.Mutex
	tip     [32]byte // current chain tip hash
	length  int      // number of entries appended
	replica string   // replica ID for this chain
}

// NewHashChain creates a chain for the given replica, starting from genesis.
func NewHashChain(replica string) *HashChain {
	genesis := sha256.Sum256([]byte("altun-forensics-genesis:" + replica))
	return &HashChain{
		tip:     genesis,
		replica: replica,
	}
}

// Append hashes a new entry into the chain and returns the new tip.
func (hc *HashChain) Append(entryJSON string) string {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	// H_i = SHA-256(H_{i-1} || entry_i)
	h := sha256.New()
	h.Write(hc.tip[:])
	h.Write([]byte(entryJSON))
	copy(hc.tip[:], h.Sum(nil))
	hc.length++
	return hex.EncodeToString(hc.tip[:])
}

// Tip returns the current chain tip as a hex string.
func (hc *HashChain) Tip() string {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	return hex.EncodeToString(hc.tip[:])
}

// Length returns the number of entries in the chain.
func (hc *HashChain) Length() int {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	return hc.length
}

// VerifyEntry checks that appending entryJSON to prevTip produces expectedTip.
// Used by the forensic verifier to validate chain integrity.
func VerifyEntry(prevTip, entryJSON, expectedTip string) bool {
	prev, err := hex.DecodeString(prevTip)
	if err != nil || len(prev) != 32 {
		return false
	}
	h := sha256.New()
	h.Write(prev)
	h.Write([]byte(entryJSON))
	actual := hex.EncodeToString(h.Sum(nil))
	return actual == expectedTip
}

// --- Integration with the existing Log* functions ---

var (
	chainsMu sync.Mutex
	chains   = make(map[string]*HashChain) // replica → chain
)

// getOrCreateChain returns the hash chain for the given replica.
func getOrCreateChain(replica string) *HashChain {
	chainsMu.Lock()
	defer chainsMu.Unlock()
	hc, ok := chains[replica]
	if !ok {
		hc = NewHashChain(replica)
		chains[replica] = hc
	}
	return hc
}

// writeJSONWithChain appends a JSON row to the forensics log and extends the
// hash chain. The chain_tip field is included in the output so the verifier
// can validate chain integrity.
func writeJSONWithChain(replica, line string) {
	if !enabled {
		return
	}
	hc := getOrCreateChain(replica)
	tip := hc.Append(line)
	// Append chain_tip to the JSON line (insert before closing brace)
	augmented := fmt.Sprintf(`%s,"chain_tip":"%s"}`, line[:len(line)-1], tip)
	mu.Lock()
	defer mu.Unlock()
	if wr == nil {
		return
	}
	_, _ = wr.WriteString(augmented)
	_, _ = wr.WriteString("\n")
	_ = wr.Flush()
}
