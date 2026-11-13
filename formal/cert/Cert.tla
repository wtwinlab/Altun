------------------------------ MODULE Cert ------------------------------
(***************************************************************************)
(* Policy-window boundary of Altun's virtual log with checkpoint            *)
(* certification.  The companion module Boundary assumes that replicas      *)
(* inside a CFT segment follow the protocol; here the Byzantine replicas    *)
(* misbehave in EVERY segment, CFT segments included, and the next segment  *)
(* may only be installed from a certified checkpoint.                       *)
(*                                                                          *)
(* Virtual log: a sequence of segments; each runs one family (CFT or BFT)   *)
(* over global positions [start, tail].  Only the last segment is active.   *)
(*                                                                          *)
(* Local commits.  Correct replica r appends ("acks") at most one value per *)
(* position of the active segment and stops after it voted to seal.  What   *)
(* r has COMMITTED locally is derived from the acks:                        *)
(*   - BFT segment: r acked <<g, v>> and a quorum of n - F replicas         *)
(*     supports <<g, v>>; the Byzantine replicas support every value at     *)
(*     every position (equivocation), as in Boundary;                       *)
(*   - CFT segment: the family's own quorum is a majority, but a CFT        *)
(*     protocol (Raft-style) commits on the leader's word: a Byzantine      *)
(*     leader can deliver different entries, and a commit index covering    *)
(*     them, to different followers.  This is over-approximated: when a     *)
(*     Byzantine replica is present (it may be the leader), every entry a   *)
(*     correct replica appended counts as locally committed.  Correct       *)
(*     replicas can therefore hold diverging prefixes L and L' in a CFT     *)
(*     segment.  Without Byzantine replicas the majority rule applies.      *)
(*                                                                          *)
(* Seal.  A quorum of the family votes; correct voters report their highest *)
(* acked position and stop acking; Byzantine voters join any seal quorum   *)
(* and may report anything, so the tail is any position >= the maximum     *)
(* reported by the correct voters.                                          *)
(*                                                                          *)
(* Certification.  After the seal at tail b, a correct replica that has     *)
(* committed the whole prefix [start, b] of the segment signs ONE           *)
(* attestation <<segment, b, prefix>>, where prefix is the sequence of      *)
(* values it committed at start..b (it stands for the prefix hash; the hash *)
(* of the previous certificate is implicit: every attestation of segment s  *)
(* is relative to the certificate segs[s].from the segment was started      *)
(* from, which is unique per s).  A Byzantine replica signs any             *)
(* attestation, including several conflicting ones; as for acks this is     *)
(* modelled implicitly by counting every Byzantine replica as a signer of   *)
(* every attestation.  A certificate for <<s, b, prefix>> exists when       *)
(* QC = N - FB replicas have signed it.  The next segment is registered     *)
(* only from a certificate: it starts at b + 1 and records the certified    *)
(* prefix (segs[s+1].from).  If no certificate forms the checkpoint does    *)
(* not advance and the segment's suffix stays provisional (no liveness is   *)
(* claimed).                                                                *)
(*                                                                          *)
(* LeaderOnly = TRUE is a control: the seal tail and the checkpoint are     *)
(* taken from a single replica's (Leader's) report, i.e. a one-signer       *)
(* certificate.                                                             *)
(***************************************************************************)
EXTENDS Naturals, FiniteSets, Sequences, TLC

CONSTANTS Replica,     \* all replicas
          Byz,         \* actually Byzantine replicas (may exceed FB: over-bound runs)
          F,           \* BFT family parameter: BFT quorum is N - F
          FB,          \* certification bound: certification quorum QC = N - FB
          Value, MaxPos, MaxSeg, FirstFamily,
          LeaderOnly,  \* TRUE: one-signer (leader report) seal + checkpoint (control)
          Leader,      \* the reporting replica when LeaderOnly
          ByzSign      \* TRUE: Byzantine replicas sign every attestation (worst case for
                       \* safety); FALSE: they sign nothing (withholding attestations)

N == Cardinality(Replica)
NB == Cardinality(Byz)
Correct == Replica \ Byz
Families == {"CFT", "BFT"}
Q(fam) == IF fam = "CFT" THEN N \div 2 + 1 ELSE N - F      \* native commit quorum
QC == N - FB                                               \* certification quorum
Pos == 1..MaxPos

VARIABLES segs,   \* sequence of [fam, start, sealed, tail, from]; from = certified prefix
                  \* (sequence of values at start-of-previous-segment .. its tail) the
                  \* segment was started from (<<>> for the first segment)
          acks,   \* set of <<s, g, r, v>>: correct replica r acked value v at position g in segment s
          votes,  \* set of <<s, r, h>>: correct replica r voted to seal s reporting highest acked h
          atts    \* set of <<s, b, r, p>>: correct replica r attested prefix p for segment s sealed at b

vars == <<segs, acks, votes, atts>>

Last == Len(segs)
Voted(s, r) == \E h \in 0..MaxPos : <<s, r, h>> \in votes
Attested(s, r) == \E a \in atts : a[1] = s /\ a[3] = r
AckedBy(s, r) == {a[2] : a \in {x \in acks : x[1] = s /\ x[3] = r}}
HighestAck(s, r) == IF AckedBy(s, r) = {} THEN segs[s].start - 1
                    ELSE CHOOSE g \in AckedBy(s, r) : \A h \in AckedBy(s, r) : h <= g
AckSet(s, g, v) == {r \in Correct : <<s, g, r, v>> \in acks}
\* Byzantine replicas count as supporting every <<g, v>> of every segment (equivocation)
ByzAcks(s, g) == IF g >= segs[s].start THEN NB ELSE 0
\* quorum commit of the native family (global view, as Committed in module Boundary)
QCommitted(s, g, v) == Cardinality(AckSet(s, g, v)) + ByzAcks(s, g) >= Q(segs[s].fam)
\* what correct replica r has committed locally (see header)
LocalCommitted(r, s, g, v) ==
    /\ <<s, g, r, v>> \in acks
    /\ \/ QCommitted(s, g, v)
       \/ segs[s].fam = "CFT" /\ NB > 0        \* Raft-style: a Byzantine leader's commit index
\* the prefix [start, b] of segment s as committed by r, if r committed all of it
HasPrefix(r, s, b) == \A g \in segs[s].start..b : \E v \in Value : LocalCommitted(r, s, g, v)
PrefixOf(r, s, b) == [i \in 1..(b - segs[s].start + 1) |->
                         CHOOSE v \in Value : LocalCommitted(r, s, segs[s].start + i - 1, v)]

\* certificates: QC signers of the same <<s, b, p>> (Byzantine replicas sign everything);
\* LeaderOnly: the single report of Leader (anything, if Leader is Byzantine)
Signers(s, b, p) == {r \in Correct : <<s, b, r, p>> \in atts}
Cert(s, b, p) ==
    IF LeaderOnly THEN Leader \in Byz \/ Leader \in Signers(s, b, p)
                  ELSE Cardinality(Signers(s, b, p)) + (IF ByzSign THEN NB ELSE 0) >= QC
\* candidate prefixes for the sealed tail of segment s
Prefixes(s) == [1..(segs[s].tail - segs[s].start + 1) -> Value]
Certs(s) == IF segs[s].sealed THEN {p \in Prefixes(s) : Cert(s, segs[s].tail, p)} ELSE {}

Init == /\ segs = << [fam |-> FirstFamily, start |-> 1, sealed |-> FALSE, tail |-> 0, from |-> <<>>] >>
        /\ acks = {}
        /\ votes = {}
        /\ atts = {}

\* a correct replica acks at most one value per position, only in the active, unsealed
\* segment, only at positions of that segment, and not after it voted to seal
Ack(r, g, v) ==
    LET s == Last IN
    /\ r \in Correct
    /\ g >= segs[s].start
    /\ ~segs[s].sealed
    /\ ~Voted(s, r)
    /\ \A w \in Value : <<s, g, r, w>> \notin acks
    /\ acks' = acks \cup {<<s, g, r, v>>}
    /\ UNCHANGED <<segs, votes, atts>>

\* seal vote: a correct replica reports its highest acked position (Byzantine votes are implicit)
Vote(r) ==
    LET s == Last IN
    /\ r \in Correct
    /\ ~segs[s].sealed
    /\ ~Voted(s, r)
    /\ votes' = votes \cup {<<s, r, HighestAck(s, r)>>}
    /\ UNCHANGED <<segs, acks, atts>>

\* the seal completes with a quorum of votes (S = correct voters; the Byzantine replicas join
\* any seal quorum); tail = max reported position, which Byzantine voters may raise.
\* LeaderOnly: the tail is whatever Leader reports.
Seal(S, t) ==
    LET s  == Last
        hs == {h \in 0..MaxPos : \E r \in S : <<s, r, h>> \in votes}
        m  == IF hs = {} THEN segs[s].start - 1 ELSE CHOOSE x \in hs : \A y \in hs : y <= x
    IN /\ ~segs[s].sealed
       /\ \A r \in S : Voted(s, r)
       /\ t >= segs[s].start - 1
       /\ IF LeaderOnly
            THEN IF Leader \in Byz THEN S = {} /\ t \in 0..MaxPos
                                   ELSE S = {Leader} /\ t = m
            ELSE /\ Cardinality(S) + NB >= Q(segs[s].fam)
                 /\ IF NB > 0 THEN t \in m..MaxPos ELSE t = m
       /\ segs' = [segs EXCEPT ![s].sealed = TRUE, ![s].tail = t]
       /\ UNCHANGED <<acks, votes, atts>>

\* certification step 1: a correct replica that committed the sealed prefix attests it (once)
Attest(r) ==
    LET s == Last
        b == segs[s].tail
    IN /\ r \in Correct
       /\ segs[s].sealed
       /\ ~Attested(s, r)
       /\ HasPrefix(r, s, b)
       /\ atts' = atts \cup {<<s, b, r, PrefixOf(r, s, b)>>}
       /\ UNCHANGED <<segs, acks, votes>>

\* certification step 2 + policy-window switch: the next segment is registered from a
\* certificate p of the sealed segment; it starts at tail + 1 and records the certified prefix
Register(fam, p) ==
    LET s == Last IN
    /\ Last < MaxSeg
    /\ fam = "BFT" => N >= 3 * F + 1
    /\ segs[s].sealed
    /\ segs[s].tail < MaxPos
    /\ segs' = Append(segs, [fam |-> fam, start |-> segs[s].tail + 1, sealed |-> FALSE,
                             tail |-> 0, from |-> p])
    /\ UNCHANGED <<acks, votes, atts>>

Next == \/ \E r \in Replica, g \in Pos, v \in Value : Ack(r, g, v)
        \/ \E r \in Replica : Vote(r)
        \/ \E S \in SUBSET Correct, t \in 0..MaxPos : Seal(S, t)
        \/ \E r \in Replica : Attest(r)
        \/ \E fam \in Families, p \in Certs(Last) : Register(fam, p)

Spec == Init /\ [][Next]_vars

Symm == Permutations(Value) \cup Permutations(Correct)

-----------------------------------------------------------------------------
(* Invariants *)

TypeOK == /\ \A i \in 1..Last : segs[i].fam \in Families /\ segs[i].from \in Seq(Value)
          /\ acks \subseteq (1..MaxSeg) \X Pos \X Correct \X Value
          /\ votes \subseteq (1..MaxSeg) \X Correct \X (0..MaxPos)
          /\ \A a \in atts : a[1] \in 1..MaxSeg /\ a[2] \in 0..MaxPos /\ a[3] \in Correct /\ a[4] \in Seq(Value)

\* (C1) CertAgreement: at most one certified prefix per (segment, tail)
CertAgreement ==
    \A s \in 1..Last : \A p1, p2 \in Certs(s) : p1 = p2

\* (C2) CertValidity: a certified prefix was committed, in full, by at least one correct
\* replica under the segment's native rule (CFT: majority / leader-driven; BFT: n - F)
CertValidity ==
    \A s \in 1..Last : \A p \in Certs(s) :
        \E r \in Correct : \A i \in 1..Len(p) : LocalCommitted(r, s, segs[s].start + i - 1, p[i])

\* NoSecondSegment: used negated, to show that a certificate can form (reachability)
NoSecondSegment == Last = 1

\* (C3) CertChain: every later segment starts at the previous tail + 1 from a certificate
\* of the previous segment, and that certificate covers exactly the previous segment
CertChain ==
    \A i \in 1..(Last - 1) :
        /\ segs[i].sealed
        /\ segs[i + 1].start = segs[i].tail + 1
        /\ segs[i + 1].from \in Certs(i)
        /\ Len(segs[i + 1].from) = segs[i].tail - segs[i].start + 1

\* value of position g in a certificate of segment s
CertValue(s, g, v) == \E p \in Certs(s) : g \in segs[s].start..segs[s].tail /\ p[g - segs[s].start + 1] = v

\* (C4) CertifiedAgreement: one value per virtual-log position across all certificates of
\* all segments, and no later segment commits anything at a certified position
CertifiedAgreement ==
    /\ \A g \in Pos, v1, v2 \in Value :
          ((\E s1 \in 1..Last : CertValue(s1, g, v1)) /\ (\E s2 \in 1..Last : CertValue(s2, g, v2))) => v1 = v2
    /\ \A s1, s2 \in 1..Last, g \in Pos, v1, v2 \in Value, r \in Correct :
          CertValue(s1, g, v1) /\ s2 > s1 /\ LocalCommitted(r, s2, g, v2) => FALSE

\* Agreement of module Boundary, restricted to BFT segments (n - F quorums intersect in a correct
\* replica).  In a CFT segment with a Byzantine replica, correct replicas may diverge: that is
\* exactly what certification must tolerate, so Agreement is only claimed for certified prefixes.
BftAgreement ==
    \A s \in 1..Last : segs[s].fam = "BFT" =>
        \A g \in Pos, v1, v2 \in Value, r1, r2 \in Correct :
            LocalCommitted(r1, s, g, v1) /\ LocalCommitted(r2, s, g, v2) => v1 = v2

\* SealFinality: a sealed segment has no quorum commit beyond its tail
SealFinality ==
    \A s \in 1..Last : segs[s].sealed =>
        \A g \in Pos, v \in Value : g > segs[s].tail => ~QCommitted(s, g, v)

\* Commits stay inside their segment
InSegment ==
    \A s \in 1..Last, g \in Pos, v \in Value, r \in Correct :
        LocalCommitted(r, s, g, v) => g >= segs[s].start

\* Local logs of the correct replicas and the certificates (for readable traces: ALIAS)
Alias == [segs |-> segs, acks |-> acks, votes |-> votes, atts |-> atts,
          localLogs |-> [r \in Correct |-> {<<s, g, v>> \in (1..Last) \X Pos \X Value : LocalCommitted(r, s, g, v)}],
          certs |-> [s \in 1..Last |-> Certs(s)]]

\* Weaker than CertValidity, used for the leader-only control to obtain the literal "L vs L'"
\* trace: once some correct replica holds a prefix through the sealed tail, every certified
\* prefix must be the prefix held by some correct replica
CertMatchesSomeCorrect ==
    \A s \in 1..Last : \A p \in Certs(s) :
        (\E r \in Correct : HasPrefix(r, s, segs[s].tail)) =>
            (\E r \in Correct : HasPrefix(r, s, segs[s].tail) /\ p = PrefixOf(r, s, segs[s].tail))

\* SealFinality restricted to BFT segments.  In a CFT segment that contains a Byzantine
\* replica two majority quorums may intersect only in that replica (e.g. N = 5: seal quorum
\* {r1, r4, byz}, commit quorum {r2, r3, byz}), so a majority "commit" beyond the sealed tail
\* is possible there; such entries are outside the certified prefix and are discarded (they
\* are not durable: a CFT protocol gives no guarantee once its assumption is broken).
SealFinalityBft ==
    \A s \in 1..Last : segs[s].fam = "BFT" /\ segs[s].sealed =>
        \A g \in Pos, v \in Value : g > segs[s].tail => ~QCommitted(s, g, v)

=============================================================================
