# Altun proxy — online switching between consensus instances

Go module `github.com/wtwinlab/altunproxy`. It implements the client-facing
side of a policy-window switch between two consensus instances that replicate
the same state machine (`../altunsm`): a Paxos instance (`cft-kv`, Altun-core)
and a chained-HotStuff instance (`hskv`, HotStuff fork), each over `n`
replicas. The proxy owns the virtual log's segment table and moves the log from
one instance to the other without stopping the service.

## Components

| Binary | Role |
|---|---|
| `altun-proxy` | single client endpoint (`POST /kv`); routes ops to the active segment; executes signed policy-window directives (`POST /admin/switch`); exposes `GET /admin/status` |
| `altun-ctl` | controller: generates ed25519 key pairs, signs single directives or a periodic switch schedule |
| `altun-load` | closed-loop client population; records every operation with invocation and response timestamps |
| `altun-check` | offline checker of a recorded history: per-key linearizability (Wing–Gong–Lowe search) and a virtual-log witness over the global positions the replicas returned |
| `directive/` | the signed directive type `{window, to, f, n, fb}` |

## Switch protocol

A directive for policy window `w` with target instance `T`, fault threshold
`f`, membership `n` and certification bound `f_B` is executed as follows.

1. **Admission.** The directive signature and window order are checked; the
   target must satisfy its family's quorum condition (`n >= 3f + 1` for BFT,
   `n >= 2f + 1` for CFT, `n >= 3 f_B + 1` when certification is enabled) and
   enough of its members must be reachable.
2. **Stop and drain.** New client ops are held; in-flight ops complete.
3. **Seal.** The old instance agrees on a `seal` entry for its segment `k`,
   which fixes the tail and the digest of the sealed state.
4. **Certification** (`-cert`). The proxy collects seal attestations
   `<k, tail, digest, prev_cert>` from the members of the old instance and
   forms a certificate out of `q_c = n - f_B` matching attestations whose
   `prev_cert` is the certificate installed last. If no certificate forms
   within `-cert-wait`, the switch aborts and the old instance resumes on the
   state attested by at least `f_B + 1` members.
5. **Transfer.** The sealed snapshot is fetched from the members and accepted
   only if its digest equals the agreed (certified) digest; it is staged on
   every member of the new instance.
6. **Activation.** The new instance agrees on a `segstart` entry carrying the
   digest and the certificate as its first entry; a replica installs the staged
   snapshot only if both match. The proxy requires `n - f` matching replies
   from a BFT instance, and with certification `q_c` signed start attestations
   from a CFT instance.
7. **Route and release.** Client traffic is routed to the new segment `k + 1`.

If any step after the seal fails, the old instance resumes as segment `k + 2`
on its unchanged sealed state (abort). The proxy's own journal is only a log;
the authoritative transition records are the replicated segment tables, from
which a restarted proxy rebuilds its state and completes an interrupted switch.

## Build

```
cd ../altunsm && go build ./...
cd ../proxy   && go build -o ../bin/ ./cmd/...
# replicas: cft-kv (cd ../.. && go build -o bin/ ./cmd/cft-kv) and hskv (HotStuff fork, cmd/hskv)
```

## Running a single-host deployment

```
cd scripts
export ALTUN_BIN=../../bin ALTUN_KEYS=/path/to/keys N=4 F=1
./genkeys.sh                    # controller key, attestation keys, HotStuff keys
./run_switching.sh /tmp/run1    # cluster + load + 20 directives + history check
CERT=1 REQUIRE_CERT=1 CTL_FB=1 ./run_switching.sh /tmp/run2   # with checkpoint certification
./scenarios.sh /tmp/out base cert_base cert_cft_lie byz_lie killleader_seal_c2b
python3 summarize.py /tmp/out/*  > summary.csv
```

`cluster.sh` starts `n` Paxos replicas, `n` HotStuff replicas and the proxy on
one host and records their pids; `run_switching.sh` adds the load generator,
the directive schedule and the offline check; `scenarios.sh` names the
fault-injection and certification scenarios (proxy crash after seal or import
with recovery, killing the old instance's contact replica while draining or
sealing, Byzantine replicas of either instance, directives exceeding the
instance's fault threshold, the unsafe modes that disable one safeguard, and
certification with `f_B = 0` or `f_B = 1` under lying CFT replicas). Each run
directory contains `history.jsonl`, `switches.jsonl`, `segments.jsonl` and
`check.json`.

## Fault injection and unsafe modes

`altun-proxy` accepts `-crash-at after-seal|after-import -crash-on K` (exit
during the K-th switch), `-hook-at stopped|drained|sealed -hook-cmd
kill-old-leader` (kill the old instance's contact replica at a switch phase),
`-fetch-from I` (fetch the snapshot from member I first) and the unsafe modes
`-unsafe-noseal`, `-unsafe-noverify`, `-unsafe-noquorum`, each of which
disables exactly one safeguard so that its contribution can be measured. The
replicas accept `-byz corrupt|lie|lie-ok|fork` and `-unsafe-nodigest` (see
`../altunsm/README.md`).

## History check

`altun-check -journal segments.jsonl history.jsonl` reports per-key
linearizability violations, duplicate or out-of-order virtual-log positions,
reads that do not return the latest put in position order, and ops whose
position lies outside the segment they report. Its exit status is non-zero if
any violation exists. `-export-csv` and `-export-meta` write the history and
the segment events in flat formats for external checkers.
