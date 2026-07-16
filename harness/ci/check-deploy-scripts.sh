#!/usr/bin/env bash
# Syntax-check all agora-de deploy shell scripts so they don't rot between
# provisioning runs. (bash -n parses without executing; safe and fast.)
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

mapfile -t scripts < <(find "$ROOT/deploy" -type f -name '*.sh' -o -type f -name 'install-*' -o -type f -name 'agora-de-*' | sort -u)
# keep only actual bash scripts (shebang grep)
scripts=()
while IFS= read -r path; do scripts+=("$path"); done < <(
  find "$ROOT/deploy" -type f \( -name '*.sh' -o -name 'install-*' -o -name 'agora-de-*' \) -print0 |
    while IFS= read -r -d '' f; do
      head -c2 "$f" 2>/dev/null | grep -q '^#!' && printf '%s\0' "$f"
    done | sort -z | tr '\0' '\n'
)

if [[ ${#scripts[@]} -eq 0 ]]; then
  echo "deploy scripts: none found" >&2
  exit 1
fi

fail=0
for script in "${scripts[@]}"; do
  if ! bash -n "$script"; then
    echo "syntax error: $script" >&2
    fail=1
  fi
done

if [[ "$fail" -ne 0 ]]; then
  exit 1
fi

echo "deploy scripts syntax: OK (${#scripts[@]} scripts)"
