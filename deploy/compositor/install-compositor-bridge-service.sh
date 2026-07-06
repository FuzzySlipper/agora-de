#!/usr/bin/env bash
set -euo pipefail
IFS=$'\n\t'

repo_root=${AGORA_DE_REPO_ROOT:-/home/dev/agora-de}
tmp_root=${AGORA_DE_TMP_ROOT:-/tmp/agora-os}
binary_tmp="$tmp_root/agora-de-compositor-bridge"
ctl_tmp="$tmp_root/agora-de-compositorctl"
binary_dest=${AGORA_DE_COMPOSITOR_BRIDGE_DEST:-/usr/local/bin/agora-de-compositor-bridge}
ctl_dest=${AGORA_DE_COMPOSITORCTL_DEST:-/usr/local/bin/agora-de-compositorctl}
ctl_user_dest=${AGORA_DE_COMPOSITORCTL_USER_DEST:-/home/agent/.local/bin/agora-de-compositorctl}
unit_dest=${AGORA_DE_COMPOSITOR_BRIDGE_UNIT:-/etc/systemd/system/compositor-bridge.service}

mkdir -p "$tmp_root"
go -C "$repo_root/go" build -trimpath -buildvcs=false -o "$binary_tmp" ./cmd/compositor-bridge
go -C "$repo_root/go" build -trimpath -buildvcs=false -o "$ctl_tmp" ./cmd/compositorctl

install -m 0755 "$binary_tmp" "$binary_dest"
install -m 0755 "$ctl_tmp" "$ctl_dest"
if id agent >/dev/null 2>&1; then
  install -D -o agent -g "$(id -gn agent)" -m 0755 "$ctl_tmp" "$ctl_user_dest"
fi
install -m 0644 "$repo_root/deploy/compositor/compositor-bridge.service" "$unit_dest"

systemctl daemon-reload
systemctl restart compositor-bridge.service
systemctl enable compositor-bridge.service >/dev/null
systemctl status compositor-bridge.service --no-pager
