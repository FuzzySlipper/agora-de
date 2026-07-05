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

## Native Window Visibility

The installed den-k8 Wayfire session can be switched to server-preferred native
window decorations with:

```bash
deploy/compositor/agora-de-wayfire-window-visibility-config enable
sudo systemctl restart agora-wayfire.service
systemctl --user restart agora-de-shellui.service agora-de-shell-background.service agora-de-shell-panel.service
```

Rollback:

```bash
deploy/compositor/agora-de-wayfire-window-visibility-config disable
sudo systemctl restart agora-wayfire.service
systemctl --user restart agora-de-shellui.service agora-de-shell-background.service agora-de-shell-panel.service
```

See `docs/den-k8-window-visibility.md` for the current behavior and live
capture evidence.
