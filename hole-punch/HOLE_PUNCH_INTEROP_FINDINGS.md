# Hole-Punch (DCUtR) Interop Findings — py-libp2p

Date: 2026-09-11. Scope: TCP / Noise / Yamux via circuit relay.

## Result matrix (unified-testing, `hole-punch`)

| # | Dialer × Listener | Relay | Result |
|---|---|---|---|
| 1 | py-v0.7.0 × py-v0.7.0 | rust-v0.56 | ✅ PASS |
| 2 | py-v0.7.0 × rust-v0.56 | rust-v0.56 | ✅ PASS |
| 3 | rust-v0.56 × py-v0.7.0 | rust-v0.56 | ✅ PASS |
| 4 | rust-v0.56 × rust-v0.56 | rust-v0.56 | ✅ PASS (control) |
| 5 | nim-v2.2 × nim-v2.2 | rust-v0.56 | ✅ PASS (control) |
| 6 | py-v0.7.0 × nim-v2.2 | rust-v0.56 | ❌ FAIL |
| 7 | nim-v2.2 × py-v0.7.0 | rust-v0.56 | ❌ FAIL |
| 8 | nim-v2.2 × py-v0.7.0 | py-v0.7.0 | ❌ FAIL |
| 9 | py-v0.7.0 × py-v0.7.0 | py-v0.7.0 | ✅ PASS |
| 10 | rust-v0.56 × rust-v0.56 | py-v0.7.0 | ✅ PASS |

Note on matrix size: `images.yaml` defines 3 peer implementations but only **2 relays (rust-v0.56, py-v0.7.0)** — there is no nim relay (the nim image comes from external GitHub and ships peer-only, no relay service wiring). So the tcp/noise/yamux cross is 3×3 peers × 2 relays = **18**, not 27. A 27-combination matrix would require adding a nim relay image + harness wiring.

Local (non-Docker) py-py relay + hole-punch smoke test (`/tmp/test_pypy_holepunch.py`): ✅ PASS.

## What was fixed in py-libp2p (branch `fix/relay-v2-spec-compliance`)

1. **TCP simultaneous open**: `SO_REUSEPORT` listeners (`libp2p/transport/tcp/tcp.py`) + source-bound dials (`dial_with_source`), threaded through `Swarm.dial_addr(..., source=...)`.
2. **Noise roles on merged connections**: new `Swarm.dial_addr_as_responder()` — the DCUtR initiator (spec server side) upgrades hole-punch dials as responder. Before: both sides initiator → `InvalidTag`.
3. **Spec compliance** (`relay/circuit_v2/dcutr.py`): CONNECT→CONNECT RTT measurement with capped ½-RTT sync wait before dialing; 4 KiB message cap; accept all valid ObsAddrs; dial every address; 3 attempts total.
4. **Address learning**: observed-addr manager matches on transport+family when NAT remaps ports; DCUtR advertises predicted `WAN_IP:listen_port`; identify reports arriving over **relayed** connections are ignored (they describe the relay pipe endpoint, not our external).
5. **Single-session discipline**: per-peer initiation lock — concurrent outbound DCUtR streams to one peer are now impossible (nim rejects them: "Already expecting an incoming connection").

Regression: `tests/core/relay`, `transport/test_tcp`, transport manager, observed-addr, `tests/core/network` — all pass (one test mock updated for the new `source` param).

## Why nim interop fails (nim-side, with evidence)

- **nim advertises the relay's own listen address as its ObsAddr.** Captured from the wire (py decoding nim's `HolePunch.CONNECT`): `ObsAddrs = [/ip4/<relay-IP>/tcp/<relay-listen-port>]`, identical to the relay MA across runs. Dialing it reaches the relay (Noise mismatch) — simultaneous open is impossible for py no matter what it does.
- Same root cause class py just fixed in itself (recording relay-pipe identify observations as externals).nim appears to do the equivalent.
- `nim→py`: py completes STOP + inbound upgrade, sends a single spec-compliant CONNECT (correct `WAN:port` addrs); nim never responds.
- Note: `unified-testing/hole-punch/images/py/node.py` still has the temporary `high_watermark=0` listener tweak, and `images/*/py-libp2p/` are temp working-tree copies currently in sync with the rfp branch — both to revert before commit.

## Next steps

1. File nim-libp2p issue with the captured advertisement bytes; re-test cross combos after their fix.
2. Push rfp branch to `sumanjeet0012/py-libp2p` and open the upstream PR (relay + DCUtR + hole-punch capability).
3. Revert temp wiring in unified-testing (`images/py/py-libp2p`, `autonat/...`, node.py tweak) before committing.
