package vplane

import (
	"crypto/ed25519"
	"encoding/json"
)

// PolicyWindow represents an operator-signed policy that determines the
// active fault model (CFT or BFT) and admissible protocol pool.
// Corresponds to σ = <mode, t_start, t_end, nonce> in the paper.
type PolicyWindow struct {
	ID        int64    `json:"id"`
	Mode      string   `json:"mode"` // "CFT" or "BFT"
	N         int      `json:"n"`    // cluster size
	F_CF      int      `json:"f_cf"` // max crash faults
	F_BF      int      `json:"f_bf"` // max Byzantine faults
	Nonce     []byte   `json:"nonce"`
	Signature []byte   `json:"signature"` // ed25519 over JSON(fields except signature)
	Protocols []string `json:"protocols"` // admissible protocol names
}

// Verify checks the operator's ed25519 signature on this policy window.
// Corresponds to Algorithm 1 line 3.
func (pw *PolicyWindow) Verify(pubKey ed25519.PublicKey) bool {
	if len(pubKey) == 0 || len(pw.Signature) == 0 {
		return false
	}
	msg := pw.signable()
	return ed25519.Verify(pubKey, msg, pw.Signature)
}

// Sign signs the policy window with the operator's private key.
func (pw *PolicyWindow) Sign(privKey ed25519.PrivateKey) {
	msg := pw.signable()
	pw.Signature = ed25519.Sign(privKey, msg)
}

// QuorumFeasible checks n >= max(3*f_BF+1, 2*f_CF+1).
// Corresponds to Algorithm 1 line 4 and assumption (A4).
func (pw *PolicyWindow) QuorumFeasible() bool {
	bftMin := 3*pw.F_BF + 1
	cftMin := 2*pw.F_CF + 1
	required := bftMin
	if cftMin > required {
		required = cftMin
	}
	return pw.N >= required
}

// AdmitsProtocol returns true if the given protocol is in this window's pool.
// Corresponds to Algorithm 1 line 6: verify π_target ∈ Π_σ.mode.
func (pw *PolicyWindow) AdmitsProtocol(protocol string) bool {
	for _, p := range pw.Protocols {
		if p == protocol {
			return true
		}
	}
	return false
}

// signable returns the deterministic byte representation for signing.
func (pw *PolicyWindow) signable() []byte {
	tmp := struct {
		ID        int64    `json:"id"`
		Mode      string   `json:"mode"`
		N         int      `json:"n"`
		F_CF      int      `json:"f_cf"`
		F_BF      int      `json:"f_bf"`
		Nonce     []byte   `json:"nonce"`
		Protocols []string `json:"protocols"`
	}{pw.ID, pw.Mode, pw.N, pw.F_CF, pw.F_BF, pw.Nonce, pw.Protocols}
	data, _ := json.Marshal(tmp)
	return data
}
