#!/usr/bin/env bash
# provision-den-k8.sh — one repo-owned, idempotent step to bring a den-k8 host to
# a known-good agora-de session state: compositor bridge + shellui binaries and
# services, Wayfire agora plugins (input-method-v1), and keybindings.
#
# Run as the desktop user (agent). The compositor-bridge system service is
# installed via the allowlisted passwordless admin helper; everything else is
# user-level. Re-running converges (managed blocks are replaced, not duplicated).
#
#   deploy/provision-den-k8.sh            provision everything
#   deploy/provision-den-k8.sh --check    verify without changing anything
set -euo pipefail
IFS=$'\n\t'

repo_root=${AGORA_DE_REPO_ROOT:-$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)}
compositor_admin=${AGORA_DE_COMPOSITOR_ADMIN:-/usr/local/sbin/agora-de-compositor-bridge-admin}
wayfire_config=${AGORA_DE_WAYFIRE_CONFIG:-$HOME/.config/wayfire.ini}
control_socket=${AGORA_DE_CONTROL_SOCKET:-/run/agent-os/compositor-control.sock}
bridge_socket=${AGORA_DE_BRIDGE_SOCKET:-/run/agent-os/compositor-bridge.sock}
shellui_url=${AGORA_DE_SHELLUI_URL:-http://127.0.0.1:17780/shell/dist/desktop/?surface=panel}
compositorctl=${AGORA_DE_COMPOSITORCTL:-/usr/local/bin/agora-de-compositorctl}

check_only=0
while [[ $# -gt 0 ]]; do
  case "$1" in
    --check) check_only=1 ;;
    -h|--help|help)
      sed -n '2,15p' "${BASH_SOURCE[0]}"
      exit 0
      ;;
    *) echo "unknown argument: $1" >&2; exit 2 ;;
  esac
  shift
done

log() { printf '\033[1m[provision]\033[0m %s\n' "$*"; }
fail() { printf '\033[1;31m[provision:fail]\033[0m %s\n' "$*" >&2; exit 1; }

# --- --check: verify only -----------------------------------------------------
if [[ "$check_only" -eq 1 ]]; then
  log "verifying agora-de session state (no changes)"
  [[ -x "$compositorctl" ]] || fail "missing compositorctl: $compositorctl"
  [[ -x "$HOME/.local/bin/agora-de-shellui" ]] || fail "missing shellui binary"
  systemctl is-active --quiet compositor-bridge.service || fail "compositor-bridge.service not active"
  systemctl --user is-active --quiet agora-de-shellui.service || fail "agora-de-shellui.service not active"
  [[ -S "$control_socket" ]] || fail "missing control socket: $control_socket"
  [[ -S "$bridge_socket" ]] || fail "missing bridge socket: $bridge_socket"
  grep -q 'input-method-v1' "$wayfire_config" || fail "input-method-v1 plugin not in $wayfire_config"
  grep -q '# >>> agora-de-keybindings' "$wayfire_config" || fail "keybindings managed block not in $wayfire_config"
  "$compositorctl" layout get >/dev/null 2>&1 || fail "compositorctl layout get failed"
  curl -sf --max-time 3 "$shellui_url" >/dev/null 2>&1 || fail "shellui panel not serving: $shellui_url"
  log "OK: binaries, services, sockets, wayfire (input-method + keybindings), compositorctl, shellui"
  exit 0
fi

# --- 1. compositor bridge (system; allowlisted sudo) --------------------------
log "installing compositor bridge (system service, allowlisted sudo)"
if [[ ! -x "$compositor_admin" ]]; then
  fail "missing admin helper $compositor_admin (run deploy/compositor/install-compositor-tools as root once)"
fi
sudo "$compositor_admin" install-bridge

# --- 2. shellui + user services ----------------------------------------------
log "installing shellui + user services"
"$repo_root/deploy/shellui/install-user-services" --enable --restart

# --- 3. wayfire input-method plugin ------------------------------------------
log "enabling Wayfire input-method-v1 plugin (+ text-input-v3)"
"$repo_root/deploy/compositor/agora-de-wayfire-input-method-config" enable

# --- 4. keybindings -----------------------------------------------------------
log "installing keybindings config + regenerating Wayfire [command] block"
"$repo_root/deploy/compositor/install-keybindings"

# --- 5. smoke test ------------------------------------------------------------
log "smoke test"
"$repo_root/deploy/provision-den-k8.sh" --check
log "done. (Wayfire reloads its config on change; if keybindings don't fire, restart Wayfire.)"
