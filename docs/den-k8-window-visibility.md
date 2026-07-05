# den-k8 Native Window Visibility

Task 4236 enabled the first server-side native window visibility slice for the
installed den-k8 Wayfire session.

## Decision

The installed Wayfire config now loads the `decoration` plugin and asks Wayfire
to prefer server-side decorations:

```ini
[core]
plugins = \
  autostart \
  command \
  foreign-toplevel \
  gtk-shell \
  place \
  move \
  resize \
  wm-actions \
  window-rules \
  decoration \
  agora-bridge

preferred_decoration_mode = server

[decoration]
title_height = 30
border_size = 4
button_order = minimize maximize close
ignore_views = none
forced_views = none
```

This is intentionally a preference, not a forced override. Apps that already
draw useful client-side chrome, such as Firefox and Chromium-family browsers,
keep their own header bars. Apps that previously appeared borderless, such as
Alacritty, foot, and Dolphin, receive a compositor title bar and border.

Layer-shell surfaces for agora-de background, panel, launcher, and operator
views are not native toplevel windows and remain undecorated.

## Apply And Roll Back

Apply the installed config:

```bash
deploy/compositor/agora-de-wayfire-window-visibility-config enable
sudo systemctl restart agora-wayfire.service
systemctl --user restart agora-de-shellui.service agora-de-shell-background.service agora-de-shell-panel.service
```

Roll back:

```bash
deploy/compositor/agora-de-wayfire-window-visibility-config disable
sudo systemctl restart agora-wayfire.service
systemctl --user restart agora-de-shellui.service agora-de-shell-background.service agora-de-shell-panel.service
```

The helper edits `/home/agent/.config/wayfire.ini` by default. Set
`AGORA_DE_WAYFIRE_CONFIG=/path/to/wayfire.ini` to test against a temp copy.

## Live Evidence

Baseline before enabling server decorations:

```text
/run/agent-os/artifacts/den-k8-window-decor-before/output-capture-1783241374920749852-6/output-capture-1783241374920749852-6.png
```

After enabling server decorations and restarting the installed session:

| App | Result | Capture |
| --- | --- | --- |
| Alacritty | Server title bar and border visible | `/run/agent-os/artifacts/den-k8-window-decor-after-Alacritty/output-capture-1783241450707169836-7/output-capture-1783241450707169836-7.png` |
| foot | Server title bar and border visible | `/run/agent-os/artifacts/den-k8-window-decor-after-foot/output-capture-1783241452880193415-8/output-capture-1783241452880193415-8.png` |
| Dolphin | Server title bar and border visible | `/run/agent-os/artifacts/den-k8-window-decor-after-dolphin/output-capture-1783241455192730454-9/output-capture-1783241455192730454-9.png` |
| Firefox | Client-side browser chrome retained, no duplicate server title bar observed | `/run/agent-os/artifacts/den-k8-window-decor-after-firefox/output-capture-1783241458131483020-10/output-capture-1783241458131483020-10.png` |
| Brave | Client-side browser chrome visible | `/run/agent-os/artifacts/den-k8-window-decor-after-brave/output-capture-1783241460860038391-11/output-capture-1783241460860038391-11.png` |
| Chromium | Client-side browser chrome visible | `/run/agent-os/artifacts/den-k8-window-decor-after-chromium/output-capture-1783241463648184925-12/output-capture-1783241463648184925-12.png` |

All six apps launched through shellui, produced `visual_inspection.status:
visible` physical output captures on `HDMI-A-1`, and were closed through
`/api/surfaces/action`.

After removing custom color overrides that Wayfire logged as invalid option
values, a clean restart produced no decoration config errors and this fresh
Alacritty capture:

```text
/run/agent-os/artifacts/den-k8-window-decor-clean/output-capture-1783241600519851782-13/output-capture-1783241600519851782-13.png
```

The native launch live harness also passed after the clean restart:

```text
/run/agent-os/artifacts/den-k8-window-decor-native-rerun/output-capture-1783241651329001772-15/output-capture-1783241651329001772-15.png
```

## Known Limits

This does not solve structured agent layout. It gives windows visible boundaries
and basic title/control affordances while task 4237 designs the stronger
agent-friendly layout model.
