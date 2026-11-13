# Formal models of the policy-window boundary

TLA+ specifications of the virtual-log boundary that Altun crosses when a
policy window changes the protocol family, together with the TLC
configurations used to check them. Both models are bounded abstractions of the
design: commits are quorum acknowledgements of log positions, a seal is one
quorum vote, and the switch decision itself is represented by its effect on the
segment table.

| Directory | Module | Scope |
|---|---|---|
| `boundary/` | `Boundary.tla` | Seal and admission check at a boundary. Byzantine replicas deviate only in BFT segments; CFT segments run under their own assumption. |
| `cert/` | `Cert.tla` | Checkpoint certification. Byzantine replicas deviate in every segment, so correct replicas of a CFT segment may hold diverging prefixes; the next segment can only be installed from a certificate. |

## Boundary model

A virtual log is a sequence of segments, each running one family (CFT or BFT)
over global positions `[start, tail]`; only the last segment is active. A
position is committed when a quorum of the family acknowledges it (CFT:
majority, BFT: `n - F`). A switch seals the active segment (a quorum votes;
every correct voter reports its highest acknowledged position and stops
acknowledging; the tail is the maximum report) and registers the next segment
at `tail + 1`. Registering a BFT segment requires `n >= 3F + 1`.

Invariants: `Agreement` (one value per position across all segments),
`SealFinality` (no commit beyond the tail of a sealed segment), `Continuity`
(consecutive segments meet at the seal point), `InSegment`, `TypeOK`.

| Configuration | Purpose |
|---|---|
| `safe_cft`, `safe_bft` | 4 replicas, 1 Byzantine, two segments starting from CFT or BFT: all invariants hold. |
| `safe_cft_3seg`, `safe_bft_3seg` | three segments (two boundaries). |
| `noquorum` | control: admission check disabled; a switch to a BFT segment with `n = 3 < 3F + 1` violates `Agreement`. |
| `noseal` | control: registration without sealing; the old instance keeps committing and `Agreement` is violated. |

## Certification model

The certification model keeps the boundary mechanics and adds the attestation
protocol. After a seal at tail `b`, every correct replica that committed the
segment's prefix through `b` signs one attestation `<<segment, b, prefix>>`.
A certificate is `QC = N - FB` attestations on the same prefix; Byzantine
replicas are counted as signers of every attestation (`ByzSign = TRUE`) or of
none (`ByzSign = FALSE`). The next segment is registered only from a
certificate and records it, so installed certificates form a chain. In a CFT
segment that contains a Byzantine replica, local commits are over-approximated
as leader-driven: anything a correct replica appended counts as committed, so
correct replicas may hold prefixes `L` and `L'` that differ. `LeaderOnly =
TRUE` replaces the certificate by a single replica's report and serves as a
control.

Invariants:

| Name | Meaning |
|---|---|
| `CertAgreement` | at most one certified prefix per (segment, tail) |
| `CertValidity` | every certified prefix was committed in full by at least one correct replica under the segment's native rule |
| `CertChain` | every later segment starts at the previous tail + 1 from a certificate of the previous segment that covers exactly that segment |
| `CertifiedAgreement` | one value per virtual-log position across all certificates; no later segment commits at a certified position |
| `BftAgreement` | correct replicas of a BFT segment agree on their local commits |
| `SealFinality`, `SealFinalityBft` | no quorum commit beyond a sealed tail (all segments / BFT segments only) |
| `InSegment`, `TypeOK` | commits stay inside their segment; typing |
| `CertMatchesSomeCorrect` | weaker than `CertValidity`; used with the leader-only control to obtain a trace in which the installed prefix is held by no correct replica |
| `NoSecondSegment` | reachability probe: its violation shows that a certificate formed and the switch completed |

Configurations (`Replica` size / Byzantine replicas / `FB`):

| Configuration | Setting | Expected outcome |
|---|---|---|
| `smoke` | 4 / 1 / 1, one position | all safety invariants hold (fast sanity check) |
| `cert_safe_cft`, `cert_safe_bft` | 4 / 1 / 1, two segments, first family CFT or BFT | all safety invariants hold |
| `cert_safe_cft_3seg`, `cert_safe_bft_3seg` | 4 / 1 / 1, three segments | all safety invariants hold across two certified boundaries |
| `cert_n5_fb1_cft`, `cert_n5_fb1_bft` | 5 / 1 / 1 | safety invariants hold with `SealFinalityBft` |
| `cert_n5_fb1_cft_sealfin`, `cert_n5_fb1_bft_sealfin` | 5 / 1 / 1 with the unrestricted `SealFinality` | `SealFinality` is violated inside the CFT segment: with 5 replicas two majority quorums may intersect only in the Byzantine replica, so a majority commit beyond the sealed tail is possible there; such entries lie outside the certified prefix and are not installed |
| `cert_n7_fb2_safe` | 7 / 2 / 2 | all safety invariants hold |
| `cert_overbound` | 4 / 2 / 1 | control: more Byzantine replicas than `FB`; `CertAgreement` is violated (two certificates whose `2 QC - N` common signers are all Byzantine) |
| `cert_n7_fb2_overbound` | 7 / 3 / 2 | control: same structure with the larger bound |
| `cert_leaderonly` | 4 / 1 / 1, `LeaderOnly` | control: a Byzantine reporter certifies two conflicting prefixes; `CertAgreement` and `CertValidity` are violated |
| `cert_leaderonly_LL` | 4 / 1 / 1, `LeaderOnly` | control: `CertMatchesSomeCorrect` is violated; the installed prefix is held by no correct replica |
| `cert_fb0_safe` | 4 / 1 / 0 (unanimity) | all safety invariants hold |
| `cert_fb0_reach` | 4 / 1 / 0, Byzantine replica signs | `NoSecondSegment` is violated: a certificate forms |
| `cert_fb0_silent` | 4 / 1 / 0, Byzantine replica withholds | `NoSecondSegment` is never violated: with unanimity one withholding replica blocks every certificate |
| `cert_fb0_silent_fb1` | 4 / 1 / 1, Byzantine replica withholds | `NoSecondSegment` is violated: `N - FB` correct replicas certify on their own |

The bound is tight: with `b` Byzantine replicas two conflicting certificates
can be assembled exactly when `b >= N - 2 FB`, which the over-bound
configurations exhibit, while a one-signer checkpoint is unsafe as soon as the
reporter is Byzantine.

## Running TLC

`tla2tools.jar` is not included; download it from the TLA+ release page and
point `TLA_JAR` at it (or place it in `tools/`).

```
./run_tlc.sh boundary safe_cft 8 8g
./run_tlc.sh cert cert_safe_cft 16 16g
./run_tlc.sh cert cert_leaderonly          # counterexample trace in cert/out/tlc_cert_leaderonly.out
python3 summarize.py cert/out boundary/out  # Markdown table of every result
```

State spaces of the small configurations have a few hundred thousand distinct
states and finish in under a minute; the `n5`, `n7` and three-segment
configurations have tens of millions of states and need several hours with 16
workers.

## Limitations

Model checking is bounded (at most 7 replicas, 2 positions, 3 segments, 2
values). Byzantine support for acknowledgements, votes and signatures is
modelled implicitly as support for everything, which is the worst case for
safety; withholding is covered separately through `ByzSign`. The CFT segment
uses a pessimistic leader-driven commit rule. Signature cryptography,
certificate dissemination, rollback of uncertified suffixes and the consensus
on the switch decision itself are outside the model.
