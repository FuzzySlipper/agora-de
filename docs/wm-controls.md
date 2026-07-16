# WM Controls (window manager for humans)

How to drive the agora-de window manager from the keyboard and the panel — move
windows between columns, swap which one is the big "master", cycle layouts, and
rebind the keys. This is the user-facing counterpart to
[`agent-surface-guide.md`](./agent-surface-guide.md) (which covers the
agent/compositorctl surface).

## TL;DR — the two questions that started this

- **"How do I make the right-column window the big one?"** → Focus it, then
  `Swap master` (`Super+Shift+Enter`) or `Move left` (`Super+Shift+H`).
- **"How do I move a window to the other column?"** → Focus it, then
  `Move left`/`Move right` (`Super+Shift+H` / `Super+Shift+L`).

## Keybindings (defaults, rebindable)

> Modifier grammar is Wayfire's: `<super>` `<ctrl>` `<alt>` `<shift>` + a key,
> e.g. `<super> <shift> KEY_H`. Bindings act on the **focused** window.

| Action | Default key | What it does |
|---|---|---|
| Focus next / prev | `Super+J` / `Super+K` | Cycle focus through the workspace's windows |
| Move focused ◀ | `Super+Shift+H` | Move toward the master column (becomes the big window when master count is 1) |
| Move focused ▶ | `Super+Shift+L` | Move toward the stack column |
| Move focused ▲ / ▼ | `Super+Shift+K` / `Super+Shift+J` | Reorder within the current column |
| Swap with master | `Super+Shift+Enter` | Exchange the focused window with the master |
| Promote to master | `Super+Shift+M` | Force the focused window to the master slot |
| Toggle floating | `Super+Shift+F` | Float / re-tile the focused window |
| Cycle layout mode | `Super+Shift+Space` | `freeform` → `zones` → `columns` |
| Cycle layout rule | `Super+Shift+R` | `master_stack` → `zones` → `dwindle` |
| Close focused | `Super+Shift+Q` | Close the focused window |
| Terminal | `Super+Enter` | (existing) launch terminal |
| Fullscreen | `Super+F` | (existing) toggle fullscreen |

### Rebinding

Keys are **not** hardcoded. Edit `~/.config/agora-de/keybindings.toml` (defaults
live at `deploy/compositor/keybindings.toml`) and regenerate the Wayfire
bindings:

```bash
python3 harness/compositor/generate-wayfire-keybindings.py --apply
```

That splices a managed block into `~/.config/wayfire.ini`'s `[command]` section
(idempotent — it replaces the previous agora block, leaving your other bindings
alone). Wayfire reloads its config on change; if a binding doesn't pick up,
restart Wayfire. Each entry is `name` / `keys` / `command` (a `compositorctl`
subcommand; use `--surface focused` to act on the focused window).

## Panel WM controls

Open the `WM` menu (lower-right). Controls show **live values** from the layout:

- `Rule: master` / `Mode: freeform` — click to cycle.
- `Master: 1` — master-column window count (click to add).
- `Ratio: 50%` — master column width share (click to step).
- `Gaps: 0` — inner/outer gaps in px (click to step).
- `Smart: on|off` — when on, a lone window gets no gaps.
- `◀ ▲ ▼ ▶` — move the focused window (same as the keyboard).
- `Swap` — swap focused with master.
- Plus `Master` (promote), `Move zone`, `Float`, `Full`, `Max`, `Min`, `Close`,
  `Reset`.

Click a taskbar entry to focus/raise a window; the WM controls then target it.

## Layout rules & modes

**Mode** (overall tiling on/off): `freeform` (no auto-tiling — windows float
where placed), `zones` (auto-tiling on), `columns` (auto-tiling, column emphasis).

**Rule** (the tiling algorithm, active in `zones`/`columns`):

- `master_stack` — one "master" column on the left, a stack of equal windows on
  the right. `Master: N` sets how many windows are in the left column; `Ratio`
  sets the left/right width split.
- `zones` — fixed named zones (primary/secondary/…); windows assign to zones.
- `dwindle` — spiral/binary split: each new window halves the last pane,
  alternating axis.

```
master_stack (Master:1, Ratio:50%)        master_stack (Master:2)
┌───────────────┬──────────────┐          ┌────────┬──────────────┐
│               │     win-b    │          │ win-a  │    win-c     │
│     win-a     ├──────────────┤          │        ├──────────────┤
│   (master)    │     win-c    │          │ win-b  │    win-d     │
│               ├──────────────┤          │        │              │
│               │     win-d    │          └────────┴──────────────┘
└───────────────┴──────────────┘
```

## Move / swap semantics (worked example)

Start: one big window on the left (`win-a`, master) + three stacked on the right
(`win-b`, `win-c`, `win-d`). Default rule `master_stack`, `Master: 1`.

**Make `win-c` the big one (swap master):** focus `win-c` → `Swap master`
(`Super+Shift+Enter`). `win-c` and `win-a` exchange places; `win-c` is now the
full-height master, `win-a` joins the right stack.

**Move `win-c` to the left column without demoting the master:** focus `win-c` →
`Move left` (`Super+Shift+H`). With `Master: 1` this also makes it the master. To
get **two on the left, two on the right**, raise `Master: 2` (panel `Master`
button, or `compositorctl layout set-settings --master-count 2`), then `Move
left`/`Move right` to place windows across the two columns.

**Reorder within a column:** focus a window → `Move up`/`Move down`
(`Super+Shift+K`/`Super+Shift+J`) to swap with its neighbour in the same column.

`Move left`/`Move right` cross the master↔stack boundary using the live master
count; `Move up`/`Move down` reorder within the current column.

## Power-user: compositorctl

Everything the panel/keys do is also on the CLI (acts on `--surface focused` when
you omit an explicit id):

```bash
# move / swap (the primitives behind the keys + panel)
agora-de-compositorctl surface move     --surface focused --direction left   # left|right|up|down
agora-de-compositorctl surface swap-master --surface focused
agora-de-compositorctl surface promote  --surface focused
agora-de-compositorctl surface focus-next          # / focus-prev

# layout
agora-de-compositorctl layout cycle-mode           # freeform/zones/columns
agora-de-compositorctl layout cycle-rule           # master_stack/zones/dwindle
agora-de-compositorctl layout set-mode --mode zones
agora-de-compositorctl layout set-settings --rule master_stack --master-count 2 --master-ratio 0.6 --inner-horizontal 8 --smart-gaps
agora-de-compositorctl layout get                  # inspect geometry/order/focused
```

## Status / gaps

- **Focus is next/prev cycling**, not geometric left/right (lateral
  focus-direction is a future primitive). `Super+J`/`Super+K` cycle the order.
- Per-window controls on the titlebar (mode-aware) are **not** yet owned by
  agora-de — the min/max/close bar today is Wayfire's built-in decoration
  (server-side). See task #5727 for the owned-SSD decision.
- Keybindings require the Wayfire `command` plugin (enabled by default) and the
  generated `[command]` block from the keymap.
