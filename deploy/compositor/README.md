# Agora DE Compositor Bridge Deployment

The compositor bridge service is owned by agora-de. It replaces the predecessor
`/usr/local/bin/compositor-bridge` service while preserving the Wayfire plugin
and control socket paths:

- `/run/agent-os/compositor-bridge.sock`
- `/run/agent-os/compositor-control.sock`

Install or update on den-k8 with:

```bash
sudo /home/dev/agora-de/deploy/compositor/install-compositor-bridge-service.sh
```

The installed unit runs `/usr/local/bin/agora-de-compositor-bridge`. The Wayfire
plugin remains the active compositor runtime dependency until the backend plugin
itself is replaced or regenerated under agora-de ownership.
