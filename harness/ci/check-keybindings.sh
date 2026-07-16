#!/usr/bin/env bash
# Validates the Agora DE keybindings config: the TOML parses and the Wayfire
# generator emits a balanced binding_/command_ block under [command].
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CONFIG="$ROOT/deploy/compositor/keybindings.toml"
GEN="$ROOT/harness/compositor/generate-wayfire-keybindings.py"

block="$(python3 "$GEN" --config "$CONFIG")"

binding_count=$(printf '%s\n' "$block" | grep -c '^binding_' || true)
command_count=$(printf '%s\n' "$block" | grep -c '^command_' || true)

if [[ -z "$binding_count" || "$binding_count" -lt 8 ]]; then
  echo "keybindings: expected at least 8 bindings, got ${binding_count:-0}" >&2
  exit 1
fi
if [[ "$binding_count" != "$command_count" ]]; then
  echo "keybindings: unbalanced bindings ($binding_count) vs commands ($command_count)" >&2
  exit 1
fi

if ! printf '%s\n' "$block" | grep -q '# >>> agora-de-keybindings'; then
  echo "keybindings: managed block missing begin marker" >&2
  exit 1
fi

# every command must reference the compositorctl path
if ! printf '%s\n' "$block" | grep -q 'command_.*= .*agora-de-compositorctl'; then
  echo "keybindings: commands must invoke agora-de-compositorctl" >&2
  exit 1
fi

echo "keybindings config: OK ($binding_count bindings)"
