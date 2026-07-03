# Agora OS Boundary

`agora-de` does not import `agora-os` internals and does not treat predecessor
log files as APIs. Governance authority remains in `agora-os`; desktop
projection and interaction live here.

## Boundary Package

The Go side exposes the current boundary vocabulary in:

```text
go/internal/osboundary
```

It defines typed clients for:

- agent summaries;
- pending admin escalations;
- escalation decisions;
- audit event streams.

The package includes `UnavailableClient`, a fail-closed implementation used
until a real typed `agora-os` service endpoint exists. It is deliberately not a
legacy adapter.

## Forbidden Couplings

The harness rejects Go source references to known predecessor coupling points:

```text
harness/policy/forbidden-patterns.txt
```

Current forbidden examples include:

- `/var/log/agent-os/admin-agent.log`
- `/var/log/agent-os/admin-human-decisions.jsonl`
- `../agora-os`
- `agora-os/internal`

Run:

```bash
./harness/ci/check-go.sh
```

The full gate also runs this check.

## Future Adapter Rule

When `agora-os` publishes the real typed API, add an adapter under a narrow Go
package that implements `osboundary.Client`. Keep conformance tests at that
adapter boundary. Do not spread governance transport details into shell,
compositor, or UI packages.

