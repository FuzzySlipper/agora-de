# Wayfire Layout Authority Probe

Status: Den task 4267 probe record.

## Sources

This probe uses local primary sources from the installed den-k8 Wayfire
development package:

- Wayfire version: `0.10.1`, package `wayfire 0.10.1-4`.
- Runtime build string: `0.10.1-746bc7e9 (Apr 6 2026) branch makepkg
  wlroots-0.19.3`.
- Headers under `/usr/include/wayfire/`.
- Simple-tile metadata at `/usr/share/wayfire/metadata/simple-tile.xml`.
- Agora-de live evidence in `docs/structured-window-handling.md`.

## Summary

Wayfire exposes enough public plugin API to attempt a narrow agora-de
layout-authority proof without synthetic keyboard shortcuts. The API surface is
strong for lifecycle, focus, workspace, output, geometry, and direct
move/resize/tile requests.

The important gap is `simple-tile`: its installed metadata exposes user
bindings and configuration, not a stable public state or command API for
agora-de zones. Treat simple-tile as live evidence that Wayfire can visibly tile
native clients, not as the final layout authority API. The next proof should
own zone placement in the agora-de Wayfire plugin or through direct Wayfire
window-manager operations, then emit the generated layout contract.

This means the Wayfire path is still viable, but only as a custom
layout-authority plugin proof. It is not viable as shell inference over
simple-tile state.

## API Surface

| Requirement | Wayfire API evidence | Classification |
| --- | --- | --- |
| Surface lifecycle | `view_mapped_signal`, `view_pre_unmap_signal`, `view_unmapped_signal`, `view_set_output_signal`; `view_interface_t::is_mapped()`, `close()`, `get_app_id()`, `get_title()`, `get_client()` | Direct |
| Post-layout geometry | `toplevel_view_interface_t::get_geometry()`, `get_pending_geometry()`, `set_geometry()`, `move()`, `resize()`; `view_geometry_changed_signal`; `toplevel_t::current().geometry` | Direct, provided the plugin listens after transactions/commits |
| Output/workarea | `output_layout_t::get_outputs()`, `output_t::get_layout_geometry()`, `output_t::get_screen_size()`, `output_workarea_manager_t::get_workarea()` | Direct |
| Workspace state | `output_t::wset()`, `workspace_set_t::get_current_workspace()`, `get_workspace_grid_size()`, `get_views()`, `get_view_main_workspace()`, `view_visible_on()`, `move_to_workspace()`, `set_workspace()`, `request_workspace()`; `workspace_changed_signal`, `workspace_set_changed_signal`, `view_change_workspace_signal` | Direct for Wayfire workspaces |
| Focus state | `seat_t::get_active_view()`, `focus_view()`, `keyboard_focus_changed_signal`; `window_manager_t::focus_request()` and `focus_raise_view()` | Direct |
| View order | `workspace_set_t::get_views(WSET_SORT_STACKING)`, `collect_views_from_scenegraph()`, `collect_views_from_output()` | Direct for stacking order; deterministic layout order must be owned by agora-de |
| Move/resize | `toplevel_view_interface_t::set_geometry()`, `move()`, `resize()`; `window_manager_t::move_request()`, `resize_request()` | Direct |
| Tile/fullscreen/minimize | `window_manager_t::tile_request()`, `fullscreen_request()`, `minimize_request()`; `view_tile_request_signal`, `view_tiled_signal`, `view_fullscreen_signal`, `view_minimized_signal` | Direct for Wayfire tile/fullscreen state |
| Zone assignment | No native Wayfire zone concept in the public API; `grid.hpp` has fixed slot helpers, simple-tile has bindings/config only | Requires agora-de plugin state/policy |
| Layout revision and command result | No built-in agora-de revision/ack model | Requires agora-de plugin state/protocol |
| Overlay annotations | Scene graph and `simple-text-node` support exist; shell overlay already proves capture-visible labels | Optional custom plugin work, not needed for first state proof |

## Simple-Tile Finding

`simple-tile.xml` exposes:

- `tile_by_default`;
- mouse bindings for move/resize;
- key bindings for toggle and directional focus;
- gap and animation settings;
- preview colors.

It does not expose a public IPC method, zone model, stable tile tree, ordered
surface list, or command/result API. The 2026-07-05 live experiment remains
valuable because it proved Wayfire can visibly place real native surfaces side
by side, but it also showed that the current bridge did not report post-layout
truth. A successor plugin should not drive simple-tile through key bindings or
read screenshots to infer geometry.

## Command Path

The next Wayfire proof can remain inside the Den 4251 churn budget if it uses
direct plugin methods:

- focus: call `wf::get_core().default_wm->focus_request()` or
  `focus_raise_view()`;
- close: call `view_interface_t::close()` and classify later unmap/stale
  events;
- move/resize: call `set_geometry()` or `move()`/`resize()` on toplevel views;
- tile/fullscreen/minimize: call `tile_request()`, `fullscreen_request()`, and
  `minimize_request()`;
- workspace: call `workspace_set_t::request_workspace()`,
  `set_workspace()`, or `move_to_workspace()` as appropriate;
- layout state: collect views from the active output workspace set, read
  current geometry after `view_geometry_changed_signal`, and maintain an
  agora-de layout revision.

The proof should return `backend_unsupported` for any operation that cannot be
implemented directly. It should not claim success for commands implemented by
synthetic key events, shell-side geometry guesses, or screenshot comparison.

## Required Plugin-Owned State

The plugin still needs small agora-de-owned state:

- stable surface ids and labels;
- mapping from surface id to requested zone id;
- deterministic surface order for tiled zones;
- layout revision;
- command result/error classification;
- pending command correlation until geometry/focus/workspace signals settle.

This is acceptable plugin state because it is backend authority state, not
shell policy. Higher-level layout policy still belongs in Rust-owned layout
model crates.

## Decision

The Wayfire proof is not impossible. Continue to Den task 4268 with a minimal
layout state bridge proof, but do not depend on simple-tile as the authority.
The first implementation should emit truthful geometry/order/workspace state
from direct Wayfire view/workspace APIs and keep zone assignment as explicit
agora-de plugin state.

Promote the Rust/Smithay path if the proof has to:

- use synthetic keyboard shortcuts for layout actions;
- infer geometry from shell state, output captures, or screenshots;
- depend on unexported simple-tile internals;
- combine shell UI policy with the Wayfire plugin;
- exceed the Den 4251 churn budget.
