---------------------------- MODULE Boundary ----------------------------
(***************************************************************************)
(* Policy-window boundary of Altun's virtual log, abstracted to            *)
(* quorum-acknowledged log positions.                                       *)
(*                                                                          *)
(* The virtual log is a sequence of segments; each segment runs one         *)
(* protocol family (CFT or BFT) over global log positions                   *)
(* [start, tail].  Only the last segment is active.  A position g is        *)
(* committed with value v in segment s when a quorum of the family acks     *)
(* <<g, v>> in s.  A policy-window switch seals the active segment (a       *)
(* quorum of the family votes; each correct voter reports its highest       *)
(* acked position and stops acking in that segment; the tail is the         *)
(* maximum reported position) and registers a new segment starting at       *)
(* tail + 1.  Registering a BFT segment requires n >= 3F + 1.               *)
(*                                                                          *)
(* Fault model: replicas in Byz are Byzantine.  In a BFT segment they are   *)
(* modelled as acking every value at every position, at any time           *)
(* (equivocation, acking after voting), and as voting with any reported    *)
(* position; this over-approximates any Byzantine behaviour for safety      *)
(* (it only adds acks).  In a CFT segment the window's assumption holds:    *)
(* replicas follow the protocol (crashes are covered because a replica may  *)
(* simply never ack or vote).  Only correct replicas' acks are stored.      *)
(*                                                                          *)
(* Switches used only for counterexamples: CheckQuorum (the n >= 3F+1       *)
(* admission check) and UseSeal (seal before registering).                  *)
(***************************************************************************)
EXTENDS Naturals, FiniteSets, Sequences, TLC

CONSTANTS Replica, Byz, F, Value, MaxPos, MaxSeg, FirstFamily, CheckQuorum, UseSeal

N == Cardinality(Replica)
Correct == Replica \ Byz
Families == {"CFT", "BFT"}
\* quorum sizes: CFT majority; BFT n - F (= 2F+1 when n = 3F+1, as PBFT/HotStuff)
Q(fam) == IF fam = "CFT" THEN N \div 2 + 1 ELSE N - F
Pos == 1..MaxPos

VARIABLES segs,      \* sequence of [fam, start, sealed, tail]
          acks,      \* set of <<s, g, r, v>>: replica r acked value v at position g in segment s
          votes      \* set of <<s, r, h>>: replica r voted to seal s, reporting highest acked position h

vars == <<segs, acks, votes>>

Last == Len(segs)
ByzActive(s) == segs[s].fam = "BFT"            \* Byzantine replicas deviate only in BFT segments
Voted(s, r) == \E h \in 0..MaxPos : <<s, r, h>> \in votes
AckedBy(s, r) == {a[2] : a \in {x \in acks : x[1] = s /\ x[3] = r}}
HighestAck(s, r) == IF AckedBy(s, r) = {} THEN segs[s].start - 1
                    ELSE CHOOSE g \in AckedBy(s, r) : \A h \in AckedBy(s, r) : h <= g
AckSet(s, g, v) == {r \in Replica : <<s, g, r, v>> \in acks}
\* Byzantine replicas of a BFT segment count as having acked every <<g, v>> of that segment
ByzAcks(s, g) == IF ByzActive(s) /\ g >= segs[s].start THEN Cardinality(Byz) ELSE 0
Committed(s, g, v) == Cardinality(AckSet(s, g, v)) + ByzAcks(s, g) >= Q(segs[s].fam)

Init == /\ segs = << [fam |-> FirstFamily, start |-> 1, sealed |-> FALSE, tail |-> 0] >>
        /\ acks = {}
        /\ votes = {}

\* a correct replica acks at most one value per position, only in the active, unsealed
\* segment, only at positions of that segment, and not after it voted to seal
Ack(r, s, g, v) ==
    \* with sealing only the last segment is ever unsealed; without it (counterexample mode)
    \* an earlier segment whose instance was never sealed keeps accepting acks
    /\ s \in 1..Last
    /\ g >= segs[s].start
    /\ r \in Correct \/ ~ByzActive(s)       \* Byzantine acks of a BFT segment are implicit
    /\ ~segs[s].sealed
    /\ ~Voted(s, r)
    /\ \A w \in Value : <<s, g, r, w>> \notin acks
    /\ acks' = acks \cup {<<s, g, r, v>>}
    /\ UNCHANGED <<segs, votes>>

\* seal vote: correct replicas report their highest acked position; Byzantine ones anything
Vote(r) ==
    LET s == Last IN
    /\ r \in Correct \/ ~ByzActive(s)
    /\ ~segs[s].sealed
    /\ ~Voted(s, r)
    /\ votes' = votes \cup {<<Last, r, HighestAck(s, r)>>}
    /\ UNCHANGED <<segs, acks>>

\* the seal completes with a quorum of votes; tail = max reported position.  In a BFT
\* segment the Byzantine replicas join any seal quorum and may raise the tail to any
\* position (they cannot lower the maximum).
Seal(S, t) ==
    LET s == Last
        byz == IF ByzActive(s) THEN Cardinality(Byz) ELSE 0
        hs == {h \in 0..MaxPos : \E r \in S : <<s, r, h>> \in votes}
        m  == IF hs = {} THEN segs[s].start - 1 ELSE CHOOSE x \in hs : \A y \in hs : y <= x
    IN /\ ~segs[s].sealed
       /\ \A r \in S : Voted(s, r)
       /\ Cardinality(S) + byz >= Q(segs[s].fam)
       /\ IF byz > 0 THEN t \in m..MaxPos ELSE t = m
       /\ segs' = [segs EXCEPT ![s].sealed = TRUE, ![s].tail = t]
       /\ UNCHANGED <<acks, votes>>

\* register the next segment (policy-window switch to family fam)
Register(fam) ==
    LET s == Last IN
    /\ Last < MaxSeg
    /\ (fam = "BFT" /\ CheckQuorum) => N >= 3 * F + 1
    /\ IF UseSeal
         THEN /\ segs[s].sealed
              /\ segs[s].tail < MaxPos
              /\ segs' = Append(segs, [fam |-> fam, start |-> segs[s].tail + 1, sealed |-> FALSE, tail |-> 0])
         \* counterexample mode: switch without sealing; the new segment starts after the
         \* highest position some replica has acked so far (a local, unagreed view)
         ELSE LET hi == {g \in Pos : \E a \in acks : a[1] = s /\ a[2] = g}
                  t  == IF hi = {} THEN segs[s].start - 1 ELSE CHOOSE m \in hi : \A x \in hi : x <= m
              IN /\ t < MaxPos
                 \* the old instance is not sealed: it keeps running (sealed stays FALSE)
                 /\ segs' = Append([segs EXCEPT ![s].tail = t],
                                   [fam |-> fam, start |-> t + 1, sealed |-> FALSE, tail |-> 0])
    /\ UNCHANGED <<acks, votes>>

Next == \/ \E r \in Replica, s \in 1..MaxSeg, g \in Pos, v \in Value : Ack(r, s, g, v)
        \/ \E r \in Replica : Vote(r)
        \/ \E S \in SUBSET Replica, t \in 0..MaxPos : Seal(S, t)
        \/ \E fam \in Families : Register(fam)

Spec == Init /\ [][Next]_vars

Symm == Permutations(Value) \cup Permutations(Correct)

-----------------------------------------------------------------------------
(* Invariants *)

TypeOK == /\ \A i \in 1..Last : segs[i].fam \in Families
          /\ acks \subseteq (1..MaxSeg) \X Pos \X Replica \X Value
          /\ votes \subseteq (1..MaxSeg) \X Replica \X (0..MaxPos)

\* (1) Agreement: one value per virtual-log position, across all segments
Agreement ==
    \A g \in Pos, v1, v2 \in Value :
        ((\E s1 \in 1..Last : Committed(s1, g, v1)) /\ (\E s2 \in 1..Last : Committed(s2, g, v2))) => v1 = v2

\* (2) Seal finality: a sealed segment has no committed position beyond its tail
SealFinality ==
    \A s \in 1..Last : segs[s].sealed =>
        \A g \in Pos, v \in Value : g > segs[s].tail => ~Committed(s, g, v)

\* (3) Continuity: consecutive segments meet exactly at the seal point
Continuity ==
    \A i \in 1..(Last - 1) : segs[i].sealed /\ segs[i + 1].start = segs[i].tail + 1

\* (4) Commits stay inside their segment
InSegment ==
    \A s \in 1..Last, g \in Pos, v \in Value : Committed(s, g, v) => g >= segs[s].start

=============================================================================
