#!/usr/bin/env bash
set -euo pipefail
IFS=$'\n\t'

repo_root=${AGORA_DE_REPO_ROOT:-$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)}
tmp_root=${AGORA_DE_TMP_ROOT:-${TMPDIR:-/tmp}/agora-de}
binary_tmp="$tmp_root/agora-de-compositor-bridge"
ctl_tmp="$tmp_root/agora-de-compositorctl"
binary_dest=${AGORA_DE_COMPOSITOR_BRIDGE_DEST:-/usr/local/bin/agora-de-compositor-bridge}
ctl_dest=${AGORA_DE_COMPOSITORCTL_DEST:-/usr/local/bin/agora-de-compositorctl}
unit_dest=${AGORA_DE_COMPOSITOR_BRIDGE_UNIT:-/etc/systemd/system/compositor-bridge.service}
systemctl_cmd=${AGORA_DE_SYSTEMCTL:-systemctl}
install_user=${AGORA_DE_INSTALL_USER:-${SUDO_USER:-}}
if [[ -z "$install_user" || "$install_user" == "root" ]]; then
  if [[ ${EUID:-$(id -u)} -ne 0 ]]; then
    install_user=$(id -un)
  elif id agent >/dev/null 2>&1; then
    install_user=agent
  fi
fi

compositor_uid=${AGORA_COMPOSITOR_UID:-}
compositor_gid=${AGORA_COMPOSITOR_GID:-}
ctl_user_dest=${AGORA_DE_COMPOSITORCTL_USER_DEST:-}
if [[ -n "$install_user" ]] && id "$install_user" >/dev/null 2>&1; then
  compositor_uid=${compositor_uid:-$(id -u "$install_user")}
  compositor_gid=${compositor_gid:-$(id -g "$install_user")}
  user_home=$(getent passwd "$install_user" | cut -d: -f6)
  ctl_user_dest=${ctl_user_dest:-$user_home/.local/bin/agora-de-compositorctl}
else
  compositor_uid=${compositor_uid:-0}
  compositor_gid=${compositor_gid:-0}
fi

mkdir -p "$tmp_root"
go -C "$repo_root/go" build -trimpath -buildvcs=false -o "$binary_tmp" ./cmd/compositor-bridge
go -C "$repo_root/go" build -trimpath -buildvcs=false -o "$ctl_tmp" ./cmd/compositorctl

install -D -m 0755 "$binary_tmp" "$binary_dest"
install -D -m 0755 "$ctl_tmp" "$ctl_dest"
if [[ -n "$install_user" ]] && id "$install_user" >/dev/null 2>&1; then
  install -D -o "$install_user" -g "$(id -gn "$install_user")" -m 0755 "$ctl_tmp" "$ctl_user_dest"
fi

unit_tmp="$tmp_root/compositor-bridge.service"
sed \
  -e "s|-g agents|-g $compositor_gid|" \
  -e "s|Environment=AGORA_COMPOSITOR_UID=1001|Environment=AGORA_COMPOSITOR_UID=$compositor_uid|" \
  -e "s|Environment=AGORA_COMPOSITOR_GID=1002|Environment=AGORA_COMPOSITOR_GID=$compositor_gid|" \
  -e "s|ExecStart=/usr/local/bin/agora-de-compositor-bridge|ExecStart=$binary_dest|" \
  "$repo_root/deploy/compositor/compositor-bridge.service" > "$unit_tmp"
install -D -m 0644 "$unit_tmp" "$unit_dest"

"$systemctl_cmd" daemon-reload
"$systemctl_cmd" restart compositor-bridge.service
"$systemctl_cmd" enable compositor-bridge.service >/dev/null
"$systemctl_cmd" status compositor-bridge.service --no-pager
