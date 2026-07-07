# Agora DE Compositor Bridge Deployment

The compositor bridge service is owned by agora-de. It replaces the predecessor
`/usr/local/bin/compositor-bridge` service while preserving the Wayfire plugin
and control socket paths:

- `/run/agent-os/compositor-bridge.sock`
- `/run/agent-os/compositor-control.sock`

Install or update on an existing Linux host with:

```bash
sudo deploy/compositor/install-compositor-bridge-service.sh
```

The installer builds and installs:

- `/usr/local/bin/agora-de-compositor-bridge`
- `/usr/local/bin/agora-de-compositorctl`
- `~/.local/bin/agora-de-compositorctl` for the invoking sudo user when a
  target user can be resolved

The installed unit runs `/usr/local/bin/agora-de-compositor-bridge`. The
Wayfire plugin remains the active compositor runtime dependency until the
backend plugin itself is replaced or regenerated under agora-de ownership.

Installer defaults are derived from the checkout and invoking user. Override
with `AGORA_DE_REPO_ROOT`, `AGORA_DE_INSTALL_USER`, `AGORA_COMPOSITOR_UID`,
`AGORA_COMPOSITOR_GID`, or `AGORA_DE_COMPOSITORCTL_USER_DEST` when rehearsing a
different host shape.

## Root Helper

Install the compositor helper once:

```bash
sudo deploy/compositor/install-compositor-tools
```

That installs `/usr/local/sbin/agora-de-compositor-bridge-admin` and a sudoers
entry allowing the invoking sudo user to run the helper without a password. Set
`AGORA_DE_SUDO_USER` to choose a different desktop user. The helper accepts only
a small command set:

```bash
sudo /usr/local/sbin/agora-de-compositor-bridge-admin install-bridge
sudo /usr/local/sbin/agora-de-compositor-bridge-admin restart-bridge
sudo /usr/local/sbin/agora-de-compositor-bridge-admin stop-bridge
sudo /usr/local/sbin/agora-de-compositor-bridge-admin status
```

## Native Window Visibility

The installed Wayfire session can be switched to server-preferred native window
decorations with:

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

See `docs/den-k8-window-visibility.md` for current den-k8 behavior and live
capture evidence from the tested host.
