// Package directive defines the signed policy-window directive that asks the
// Altun proxy to switch protocol family at a policy-window boundary.
package directive

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
)

// Directive is issued by the controller (policy layer) for policy window Window.
type Directive struct {
	Window uint64 `json:"window"`       // policy-window id, strictly increasing
	To     string `json:"to"`           // target instance: "paxos" | "hotstuff"
	F      int    `json:"f"`            // fault threshold assumed for the window
	N      int    `json:"n"`            // membership size of the target instance
	FB     *int   `json:"fb,omitempty"` // certification fault bound f_B (nil: f_B = f; 0: unanimity)
	Ts     int64  `json:"ts"`           // issue time (informational)
	Sig    string `json:"sig,omitempty"`
}

func (d Directive) payload() []byte {
	d.Sig = ""
	b, _ := json.Marshal(d)
	return b
}

// Sign signs d with priv.
func (d *Directive) Sign(priv ed25519.PrivateKey) {
	d.Sig = base64.StdEncoding.EncodeToString(ed25519.Sign(priv, d.payload()))
}

// CertBound returns f_B: the directive's value, or f when it carries none.
func (d Directive) CertBound() int {
	if d.FB != nil {
		return *d.FB
	}
	return d.F
}

// Verify checks the signature against pub.
func (d Directive) Verify(pub ed25519.PublicKey) error {
	sig, err := base64.StdEncoding.DecodeString(d.Sig)
	if err != nil || !ed25519.Verify(pub, d.payload(), sig) {
		return errors.New("bad directive signature")
	}
	return nil
}

// GenKeys writes a fresh key pair to path.key / path.pub.
func GenKeys(path string) error {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path+".key", []byte(base64.StdEncoding.EncodeToString(priv)), 0o600); err != nil {
		return err
	}
	return os.WriteFile(path+".pub", []byte(base64.StdEncoding.EncodeToString(pub)), 0o644)
}

func readB64(p string) ([]byte, error) {
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	return base64.StdEncoding.DecodeString(string(b))
}

// LoadPriv / LoadPub read keys written by GenKeys.
func LoadPriv(p string) (ed25519.PrivateKey, error) {
	b, err := readB64(p)
	return ed25519.PrivateKey(b), err
}
func LoadPub(p string) (ed25519.PublicKey, error) {
	b, err := readB64(p)
	return ed25519.PublicKey(b), err
}
