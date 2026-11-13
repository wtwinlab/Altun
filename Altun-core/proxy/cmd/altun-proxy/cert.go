package main

// Checkpoint certification of the sealed state.
//
// After the seal the proxy asks every member of the old instance for its
// signed seal attestation <k, tail, digest, prev_cert> and forms a certificate
// out of q_c = n - f_B valid attestations from distinct members with identical
// (tail, digest, prev_cert) whose prev_cert equals the certificate the proxy
// installed last.  Only a certified checkpoint is fetched, imported and
// activated; the certificate travels in the segstart entry.  If no
// certificate forms within -cert-wait the switch aborts and the old instance
// resumes.  The activation acknowledgement of the new instance uses the same
// quorum: q_c signed start attestations <k+1, tail, digest, hash(cert)>.

import (
	"crypto/ed25519"
	"fmt"
	"strings"
	"time"

	"github.com/wtwinlab/altunsm"
)

type attGot struct {
	m   int
	a   altunsm.Attestation
	err error
	t   int64
}

// gatherAttestations asks every member of b for its attestation of (kind,
// seg), retrying members that have not reached that point yet (409), until
// deadline.  Results are streamed on the returned channel (one per member).
func (p *Proxy) gatherAttestations(b Backend, kind string, seg uint64, deadline time.Time) <-chan attGot {
	ch := make(chan attGot, b.N())
	for m := 0; m < b.N(); m++ {
		go func(m int) {
			var err error
			for {
				var a altunsm.Attestation
				to := time.Until(deadline)
				if to <= 0 {
					break
				}
				if to > 2*time.Second {
					to = 2 * time.Second
				}
				if _, err = getJSON(fmt.Sprintf("%s/altun/attest?kind=%s&seg=%d", b.Admin()[m], kind, seg), &a, to); err == nil {
					ch <- attGot{m: m, a: a, t: now()}
					return
				}
				if !strings.Contains(err.Error(), "409") {
					break
				}
				time.Sleep(10 * time.Millisecond)
			}
			if err == nil {
				err = fmt.Errorf("deadline")
			}
			ch <- attGot{m: m, err: err, t: now()}
		}(m)
	}
	return ch
}

// certOutcome is the result of one attestation collection.
type certOutcome struct {
	matched  int                  // attestations in the winning group
	rejected []string             // members whose attestation was not counted, with the reason
	groups   map[string]int       // agreement key -> count (valid signatures only)
	best     *altunsm.Attestation // representative of the largest group
	bestN    int
	byKey    map[string][]altunsm.Attestation
	att      []altunsm.Attestation // attestations of the winning group
}

// collect gathers attestations of (kind, seg) from b and stops as soon as
// some group of qc matching valid attestations exists, or as soon as no group
// can reach qc any more, or at the deadline.  prev is the expected prev_cert.
func (p *Proxy) collect(b Backend, kind string, seg uint64, prev string, qc int, deadline time.Time) certOutcome {
	out := certOutcome{groups: map[string]int{}}
	byKey := map[string][]altunsm.Attestation{}
	out.byKey = byKey
	ch := p.gatherAttestations(b, kind, seg, deadline)
	n := b.N()
	seen := map[string]bool{}
	bad := 0
	got := 0
	for i := 0; i < n; i++ {
		g := <-ch
		got++
		name := fmt.Sprintf("%s%d", b.Name(), g.m+1)
		why := ""
		switch {
		case g.err != nil:
			why = "no attestation: " + g.err.Error()
		case g.a.Member != name:
			why = "member name " + g.a.Member
		case seen[name]:
			why = "duplicate member"
		case !g.a.Verify(p.certPubs[name]):
			why = "bad signature"
		case g.a.Kind != kind || g.a.Seg != seg:
			why = fmt.Sprintf("attests %s %d", g.a.Kind, g.a.Seg)
		case g.a.PrevCert != prev:
			why = fmt.Sprintf("prev_cert %.12s != installed %.12s", g.a.PrevCert, prev)
		}
		if why != "" {
			bad++
			out.rejected = append(out.rejected, fmt.Sprintf("%s: %s", name, why))
			p.j.Append(map[string]any{"kind": "attestation-rejected", "phase": kind, "member": name, "seg": seg, "why": why, "t": now()})
		} else {
			seen[name] = true
			k := g.a.Key()
			byKey[k] = append(byKey[k], g.a)
			out.groups[k]++
			if out.groups[k] > out.bestN {
				out.bestN, out.best = out.groups[k], &byKey[k][0]
			}
		}
		if out.bestN >= qc {
			break
		}
		// otherwise keep collecting until every member answered or the
		// deadline passed: the abort path resumes on the state attested by
		// at least f_B+1 members, which needs the complete picture
	}
	if out.bestN >= qc {
		out.matched = out.bestN
		out.att = byKey[out.best.Key()]
		if got < n {
			// the quorum is complete: the remaining replies are journaled
			// asynchronously (they do not sit on the switch path)
			best := out.best.Key()
			go func(rest int) {
				for i := 0; i < rest; i++ {
					g := <-ch
					name := fmt.Sprintf("%s%d", b.Name(), g.m+1)
					why := "late, agrees with the certified group"
					switch {
					case g.err != nil:
						why = "no attestation: " + g.err.Error()
					case g.a.Member != name || !g.a.Verify(p.certPubs[name]):
						why = "bad signature or member name"
					case g.a.Key() != best:
						why = "disagrees with the certified group"
					}
					ev := "attestation-rejected"
					if g.err == nil && g.a.Key() == best {
						ev = "attestation-late" // valid and agreeing, only after the quorum was complete
					}
					p.j.Append(map[string]any{"kind": ev, "phase": kind, "member": name, "seg": seg, "why": why, "late": true, "tail": g.a.Tail, "digest": g.a.Digest, "t": now()})
				}
			}(n - got)
		}
		for k, as := range byKey {
			if k != out.best.Key() {
				for _, a := range as {
					out.rejected = append(out.rejected, fmt.Sprintf("%s: %s %d tail=%d digest=%.12s (minority)", a.Member, a.Kind, a.Seg, a.Tail, a.Digest))
					p.j.Append(map[string]any{"kind": "attestation-rejected", "phase": kind, "member": a.Member, "seg": seg, "why": "disagrees with the certified group", "tail": a.Tail, "digest": a.Digest, "t": now()})
				}
			}
		}
	} else {
		out.matched = out.bestN
	}
	return out
}

// resumeDigest chooses the state the old instance resumes on when no
// certificate formed: the sealed state attested by the largest group of at
// least f_B+1 members (so containing at least one correct one); on a tie, the
// group that agrees with the seal reply.  If no group qualifies, the seal
// reply's state.
func (oc certOutcome) resumeDigest(fb int, sealTail uint64, sealDigest string) (uint64, string) {
	var best *altunsm.Attestation
	bestN := 0
	for k, as := range oc.byKey {
		n := oc.groups[k]
		if n < fb+1 {
			continue
		}
		if n > bestN || (n == bestN && as[0].Digest == sealDigest) {
			best, bestN = &as[0], n
		}
	}
	if best == nil {
		return sealTail, sealDigest
	}
	return best.Tail, best.Digest
}

// certify forms the checkpoint certificate of sealed segment k.  It returns
// nil and the reason if fewer than q_c matching attestations arrived.
func (p *Proxy) certify(old Backend, k uint64, fb, n int, rep *SwitchReport) (*altunsm.Certificate, certOutcome, string) {
	qc := n - fb
	t0 := now()
	oc := p.collect(old, "seal", k, p.certHash, qc, time.Now().Add(p.certWait))
	rep.CertMatched, rep.CertRejected, rep.CertQuorum = oc.matched, oc.rejected, qc
	if oc.matched < qc {
		p.j.Append(map[string]any{"kind": "cert-failed", "seg": k, "t0": t0, "t": now(), "matched": oc.matched, "need": qc, "groups": oc.groups, "rejected": oc.rejected})
		return nil, oc, fmt.Sprintf("certification: %d matching attestations, need %d (groups %v)", oc.matched, qc, oc.groups)
	}
	c := &altunsm.Certificate{Seg: k, Tail: oc.best.Tail, Digest: oc.best.Digest, PrevCert: p.certHash, N: n, FB: fb, Att: oc.att}
	if err := c.Verify(p.certPubs, p.certHash); err != nil {
		return nil, oc, "certification: " + err.Error()
	}
	p.j.Append(map[string]any{"kind": "certified", "seg": k, "t0": t0, "t": now(), "tail": c.Tail, "digest": c.Digest, "cert": c.Hash(), "prev_cert": c.PrevCert, "matched": oc.matched, "need": qc, "rejected": oc.rejected})
	return c, oc, ""
}

// loadCertPubs reads the members' public keys (NAME.pub) for certification.
func loadCertPubs(dir string) map[string]ed25519.PublicKey {
	pubs, err := altunsm.LoadPubs(dir)
	if err != nil {
		panic(err)
	}
	return pubs
}

// installedCert is the hash of the certificate recorded with the last
// activation in a replicated segment table (used on recovery).
func installedCert(table []altunsm.Record) string {
	for i := len(table) - 1; i >= 0; i-- {
		if table[i].Kind == "activate" && table[i].Cert != "" {
			return table[i].Cert
		}
	}
	return ""
}
