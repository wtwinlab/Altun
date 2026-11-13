package altunsm

// Checkpoint certification.
//
// After a segment is sealed, every replica of the old instance signs one
// attestation <kind=seal, segment k, tail, digest(sealed state),
// hash(previous certificate)> with its own ed25519 key.  The proxy forms a
// certificate out of q_c = n - f_B attestations from distinct members with
// identical (tail, digest, prev_cert) whose prev_cert equals the certificate
// installed with segment k.  The certificate travels inside the segstart
// entry of the new instance; a replica installs the staged snapshot only if
// the certificate is valid and certifies the digest of that snapshot.  After
// the activation every replica signs <kind=start, segment k+1, tail, digest,
// hash(certificate)>; the proxy needs q_c of those as well.
//
// A correct replica signs at most one attestation per (kind, segment): the
// first one is cached and returned again on every later request.

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Attestation is one replica's signed statement about a checkpoint.
type Attestation struct {
	Kind     string `json:"kind"`      // seal | start
	Seg      uint64 `json:"seg"`       // seal: the sealed segment k; start: the started segment k+1
	Tail     uint64 `json:"tail"`      // last global position of the sealed state
	Digest   string `json:"digest"`    // digest of the sealed state
	PrevCert string `json:"prev_cert"` // seal: hash of the certificate installed with segment k; start: hash of the certificate carried by segstart
	Member   string `json:"member"`    // e.g. paxos1, hotstuff3
	Sig      string `json:"sig,omitempty"`
}

func (a Attestation) payload() []byte {
	a.Sig = ""
	b, _ := json.Marshal(a)
	return b
}

// Sign signs a with priv.
func (a *Attestation) Sign(priv ed25519.PrivateKey) {
	a.Sig = base64.StdEncoding.EncodeToString(ed25519.Sign(priv, a.payload()))
}

// Verify checks the signature of a against pub.
func (a Attestation) Verify(pub ed25519.PublicKey) bool {
	sig, err := base64.StdEncoding.DecodeString(a.Sig)
	return err == nil && len(pub) == ed25519.PublicKeySize && ed25519.Verify(pub, a.payload(), sig)
}

// Key returns the agreement key of an attestation.
func (a Attestation) Key() string {
	return fmt.Sprintf("%s|%d|%d|%s|%s", a.Kind, a.Seg, a.Tail, a.Digest, a.PrevCert)
}

// Certificate is q_c = N - FB matching seal attestations of segment Seg.
type Certificate struct {
	Seg      uint64        `json:"seg"`
	Tail     uint64        `json:"tail"`
	Digest   string        `json:"digest"`
	PrevCert string        `json:"prev_cert"`
	N        int           `json:"n"`
	FB       int           `json:"fb"`
	Att      []Attestation `json:"att"`
}

// Hash is the SHA-256 of the canonical JSON encoding of the certificate.
func (c *Certificate) Hash() string {
	b, _ := json.Marshal(c)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// Quorum is q_c = n - f_B.
func (c *Certificate) Quorum() int { return c.N - c.FB }

// Verify checks that c holds at least q_c valid seal attestations from
// distinct members, all over (Seg, Tail, Digest, PrevCert), and that PrevCert
// equals prev (the certificate installed before it).
func (c *Certificate) Verify(pubs map[string]ed25519.PublicKey, prev string) error {
	if c == nil {
		return errors.New("no certificate")
	}
	if c.FB < 0 || c.N < 1 || (c.FB > 0 && c.N < 3*c.FB+1) {
		return fmt.Errorf("certificate n=%d f_B=%d violates n >= 3f_B+1", c.N, c.FB)
	}
	if c.PrevCert != prev {
		return fmt.Errorf("certificate chains to %.12s, installed is %.12s", c.PrevCert, prev)
	}
	seen := map[string]bool{}
	good := 0
	for _, a := range c.Att {
		if a.Kind != "seal" || a.Seg != c.Seg || a.Tail != c.Tail || a.Digest != c.Digest || a.PrevCert != c.PrevCert {
			continue
		}
		pub, ok := pubs[a.Member]
		if !ok || seen[a.Member] || !a.Verify(pub) {
			continue
		}
		seen[a.Member] = true
		good++
	}
	if good < c.Quorum() {
		return fmt.Errorf("certificate has %d valid attestations, need %d", good, c.Quorum())
	}
	return nil
}

// ---------------------------------------------------------------- keys

func readB64(p string) ([]byte, error) {
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	return base64.StdEncoding.DecodeString(strings.TrimSpace(string(b)))
}

// LoadPubs reads every NAME.pub in dir (ed25519, base64, as written by
// altun-ctl -genkeys) into a map keyed by NAME.
func LoadPubs(dir string) (map[string]ed25519.PublicKey, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.pub"))
	if err != nil {
		return nil, err
	}
	out := map[string]ed25519.PublicKey{}
	for _, f := range files {
		b, err := readB64(f)
		if err != nil {
			return nil, err
		}
		out[strings.TrimSuffix(filepath.Base(f), ".pub")] = ed25519.PublicKey(b)
	}
	return out, nil
}

// LoadPriv reads dir/name.key.
func LoadPriv(dir, name string) (ed25519.PrivateKey, error) {
	b, err := readB64(filepath.Join(dir, name+".key"))
	if err != nil {
		return nil, err
	}
	if len(b) != ed25519.PrivateKeySize {
		return nil, errors.New("bad private key size")
	}
	return ed25519.PrivateKey(b), nil
}

// UseKeys gives the replica its signing identity (dir/name.key) and the
// public keys of every member of both instances (dir/*.pub).
func (m *SM) UseKeys(dir, name string) error {
	priv, err := LoadPriv(dir, name)
	if err != nil {
		return err
	}
	pubs, err := LoadPubs(dir)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Name, m.priv, m.pubs = name, priv, pubs
	return nil
}

// installedCertLocked is the hash of the certificate installed with the
// current segment: the last activate record of the table that carries one.
func (m *SM) installedCertLocked() string { return installedCertOf(m.table) }

func installedCertOf(table []Record) string {
	for i := len(table) - 1; i >= 0; i-- {
		if table[i].Kind == "activate" && table[i].Cert != "" {
			return table[i].Cert
		}
	}
	return ""
}

// Attest returns this replica's signed attestation of kind for segment seg
// (seal: seg must be the sealed segment; start: seg must be the active
// segment).  It signs at most once per (kind, seg).
func (m *SM) Attest(kind string, seg uint64) (Attestation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.priv == nil {
		return Attestation{}, errors.New("replica has no signing key")
	}
	key := fmt.Sprintf("%s|%d", kind, seg)
	if a, ok := m.attCache[key]; ok {
		return a, nil
	}
	var a Attestation
	switch kind {
	case "seal":
		if m.state != Sealed || m.seg != seg {
			return Attestation{}, fmt.Errorf("not sealed at %d: %s %d", seg, m.state, m.seg)
		}
		r := m.sealReply() // Byzantine replicas report their lie here as well
		a = Attestation{Kind: "seal", Seg: seg, Tail: r.Tail, Digest: r.Digest, PrevCert: m.installedCertLocked(), Member: m.Name}
	case "start":
		if m.state != Active || m.seg != seg {
			return Attestation{}, fmt.Errorf("not active at %d: %s %d", seg, m.state, m.seg)
		}
		last := m.table[len(m.table)-1]
		a = Attestation{Kind: "start", Seg: seg, Tail: m.start - 1, Digest: last.Digest, PrevCert: m.installedCertLocked(), Member: m.Name}
		if m.Byz == ByzLie {
			a.Digest = "bad" + a.Digest[3:]
		}
	default:
		return Attestation{}, errors.New("unknown attestation kind")
	}
	a.Sign(m.priv)
	if m.attCache == nil {
		m.attCache = map[string]Attestation{}
	}
	m.attCache[key] = a
	return a, nil
}
