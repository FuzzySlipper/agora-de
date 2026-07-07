# Agora DE Theme Boundary

Agora DE theme authority starts in `go/internal/shellui/theme` and is mirrored
for TypeScript shell code by `@agora-de/theme`.

The token contract is intentionally narrow:

- normal presentation uses `presentation`, `state`, `layout`, and `typography`
  tokens;
- shell chrome uses `component` tokens for named panel, popup, overlay, and
  taskbar state treatments;
- physical-display evidence uses `evidence` tokens;
- arbitrary CSS, layout-capable theme overrides, imports, and URL-bearing values
  remain outside the supported customization surface.

## Authoritative Tokens

The default dark manifest is represented by `theme.DefaultTokenDefinitions()`,
`theme.DefaultThemeID` (`agora-default`), and
`harness/fixtures/theme/agora-default-theme.json`. Go-rendered fallback/live
shell HTML resolves a `theme.Selection` at shellui handler startup and consumes
`var(--agora-*)` tokens rather than owning colors, radii, and panel dimensions
directly.

Bundled variants live in `go/internal/shellui/theme` alongside fixtures under
`harness/fixtures/theme/`. `agora-ember` is the first bundled variant. It keeps
the same evidence markers and layout dimensions while changing presentation
colors.

Component tokens such as `--agora-panel-bg`, `--agora-popup-shadow`,
`--agora-overlay-label-bg`, and `--agora-taskbar-minimized-bg` give visual
polish a central vocabulary without moving style choices into compositor,
layout, or launch plumbing.

TypeScript feature libraries consume the same token vocabulary through
`@agora-de/theme`. Feature packages may expose component-specific token maps,
such as `taskbarThemeVars` or `appLauncherThemeVars`, but they should not invent
unrelated visual constants in feature code.

## Evidence Tokens

The following tokens are stable evidence markers:

- `--agora-evidence-bg`
- `--agora-evidence-accent`
- `--agora-evidence-strong`

The live physical-output classifier in `harness/live/check-den-k8.py` keys on
these visible markers through the `agora-de.theme-evidence.v1` contract and can
recognize light or dark text presentation. Normal theme iteration can change
`--agora-bg`, `--agora-accent`, borders, typography, and surfaces without
necessarily changing the classifier, but evidence token changes require
updating the live evidence classifier and fixtures together.

## Selection Path

Shellui accepts two theme selection knobs:

- `AGORA_DE_SHELLUI_THEME_ID` / `--theme-id` selects a bundled theme id. Empty
  means `agora-default`.
- `AGORA_DE_SHELLUI_THEME_MANIFEST` / `--theme-manifest` loads a JSON manifest
  from disk. A manifest path takes precedence over a bundled id.

User manifests are validated with `DecodeManifest`, overlaid onto the default
token set, and rendered with `SafeTokenCSS`. This lets users change a small
number of tokens without copying the whole bundled manifest, while preserving
safe default values for required shell dimensions and evidence markers.

Shellui resolves configured themes through a safe runtime fallback. Invalid
theme ids, missing manifest paths, or invalid manifest content fall back to
`agora-default`; `/api/theme` reports the active id, source, whether fallback
occurred, and the fallback reason.

## Customization Path

User-facing theme customization validates manifests through
`go/internal/shellui/theme.DecodeManifest` and generates CSS with
`SafeTokenCSS`. The sanitizer accepts only `--agora-*` token declarations and
rejects layout/exfiltration-oriented CSS fragments. This keeps visual
customization separate from compositor, session, launch, and governance
plumbing.
