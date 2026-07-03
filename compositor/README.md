# Compositor Work Area

Wayfire is the initial backend because it is the predecessor path with working
behavior. The repo still treats it as one backend behind the compositor backend
API, not as the architecture.

- `wayfire-plugin/`: future lifted plugin source.
- `protocol-fixtures/`: socket and protocol conformance fixtures.
- `standard-protocol-probe/`: probe for security-context, foreign-toplevel, and
  image-copy-capture support.
- `smithay-spike/`: custom compositor feasibility work.

The capability matrix lives at
`protocol-fixtures/capabilities/backend-capability-matrix.json`. It keeps the
current decision explicit: standard protocols cover useful surface/capture
territory, while synchronous input deny and geometry control still need a
compositor-specific enforcement point until a spike proves otherwise.

Wayfire socket protocol JSONL fixtures live under `protocol-fixtures/wayfire/`.
They are consumed by `go/internal/wayfireproto` tests and checked directly by
the compositor harness.
