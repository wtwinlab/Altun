# altunsm — replicated state machine of the switching stack

Go module `github.com/wtwinlab/altunsm`. It is the deterministic key-value
state machine that every consensus instance behind the Altun proxy executes:
the Paxos replicas (`cft-kv` in Altun-core) and the HotStuff replicas (`hskv`
in the HotStuff fork) apply the same transition function to the commands their
protocol orders. Because the state is identical on both sides, a snapshot taken
at a policy-window boundary can be installed by the other instance.

## Commands

`Op` is the command type (JSON). Client commands are `get` and `put`; a put
returns the value it overwrote, and a per-client dedup table makes retries
idempotent. Three commands implement the boundary of the virtual log:

| Command | Agreed by | Effect |
|---|---|---|
| `seal` | the old instance | stops execution of client ops, fixes the segment tail, appends a transition record to the replicated segment table and freezes the sealed state together with its digest |
| `segstart` | the new instance, as its first entry | installs the snapshot staged out of band only if its digest equals the digest carried by the entry (and, when a certificate is required, only if the entry carries a valid checkpoint certificate for that digest); otherwise the replica marks itself diverged and serves nothing |
| `peek` | the old instance | captures the state without sealing; used only by the proxy mode that demonstrates the consequence of skipping the seal |

The segment table, the KV map and the dedup table are part of every snapshot,
so the complete transition history travels across boundaries and can be
replayed from any active instance.

## Checkpoint certification

`attest.go` implements the attestation side of checkpoint certification. After
a seal, a replica signs one `Attestation` of kind `seal` for the sealed
segment: `<segment, tail, digest, prev_cert, member>` with its ed25519 key,
where `prev_cert` is the hash of the certificate installed with that segment.
After an activation it signs one attestation of kind `start` for the new
segment. A replica signs at most once per (kind, segment); later requests
return the cached attestation.

A `Certificate` is `q_c = n - f_B` attestations from distinct members with
identical `(segment, tail, digest, prev_cert)`. `Certificate.Verify` checks
`n >= 3 f_B + 1`, the signatures, member distinctness and that `prev_cert`
equals the certificate installed before it, so certificates form a chain. With
`RequireCert` set, `segstart` installs a snapshot only with a valid certificate
that covers the entry's digest.

Keys are base64-encoded ed25519 keys in `NAME.key` / `NAME.pub` files;
`UseKeys(dir, name)` loads the replica's own key and the public keys of every
member (`altun-ctl -genkeys DIR/NAME` writes a pair).

## HTTP endpoints

`RegisterHTTP` adds the out-of-band endpoints the proxy uses:

| Endpoint | Purpose |
|---|---|
| `GET /altun/status` | state, segment, position and the replicated segment table |
| `GET /altun/snapshot[?seg=K]` | the sealed state and its digest (`Altun-Digest` header) |
| `POST /altun/import` | stage a snapshot for a new segment |
| `GET /altun/attest?kind=seal\|start&seg=K` | the signed attestation (409 until the replica reached that point) |

## Fault injection

`SM.Byz` selects a Byzantine behaviour for experiments: `corrupt` (imports a
corrupted snapshot and reports honestly), `lie` (reports wrong digests and
serves a corrupted snapshot), `lie-ok` (corrupts the imported state but claims
the expected digest) and `fork` (presents an older prefix of its own log at the
seal, consistently in its reply, its snapshot and its attestation).
`NoDigestCheck` disables the segstart digest comparison.
