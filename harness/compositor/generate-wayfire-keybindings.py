#!/usr/bin/env python3
"""Generate Wayfire [command] bindings from the Agora DE keybindings config.

Reads a TOML keymap (default: deploy/compositor/keybindings.toml, or the
installed copy at ~/.config/agora-de/keybindings.toml) and emits Wayfire
``[command]`` binding lines:

    binding_<name> = <keys>
    command_<name> = <compositorctl> <command>

Modes:
  (default)  print the managed binding block to stdout.
  --apply    splice the managed block into the Wayfire config (replacing any
             existing managed block). The managed block is delimited by marker
             comments so non-agora bindings (e.g. binding_terminal) are preserved.

The keymap is the single source of truth; rebinding is "edit the TOML, re-run
--apply" — no Wayfire plugin edits required.
"""
from __future__ import annotations

import argparse
import pathlib
import sys

try:
    import tomllib  # Python 3.11+
except ModuleNotFoundError:  # pragma: no cover
    print("error: this script needs Python 3.11+ (tomllib)", file=sys.stderr)
    raise

BEGIN = "# >>> agora-de-keybindings (managed - do not edit; regenerate via generate-wayfire-keybindings.py)"
END = "# <<< agora-de-keybindings"


def default_config() -> pathlib.Path:
    repo = pathlib.Path(__file__).resolve().parents[2]
    candidate = pathlib.Path.home() / ".config" / "agora-de" / "keybindings.toml"
    if candidate.exists():
        return candidate
    return repo / "deploy" / "compositor" / "keybindings.toml"


def render_block(config_path: pathlib.Path) -> str:
    data = tomllib.loads(config_path.read_text())
    compositorctl = data.get("compositorctl", "agora-de-compositorctl")
    bindings = data.get("binding", [])
    lines = [BEGIN, "# source: {}".format(config_path)]
    for entry in bindings:
        name = entry["name"]
        keys = entry["keys"]
        command = entry["command"]
        lines.append("binding_{} = {}".format(name, keys))
        lines.append("command_{} = {} {}".format(name, compositorctl, command))
    lines.append(END)
    return "\n".join(lines) + "\n"


def splice(wayfire_ini: pathlib.Path, block: str) -> bool:
    if not wayfire_ini.exists():
        print("error: wayfire config not found: {}".format(wayfire_ini), file=sys.stderr)
        return False
    text = wayfire_ini.read_text()
    # 1. drop any existing managed block (anywhere) so re-apply is idempotent.
    if BEGIN in text and END in text:
        before, _, rest = text.partition(BEGIN)
        _, _, after = rest.partition(END)
        text = before + after
    # 2. insert the fresh block inside the [command] section (right after its
    #    header) so Wayfire parses the binding_* keys under [command].
    lines = text.splitlines(keepends=True)
    insert_at = None
    for index, line in enumerate(lines):
        if line.strip() == "[command]":
            insert_at = index + 1
            break
    if insert_at is None:
        # no [command] section yet: create one.
        text = text.rstrip() + "\n\n[command]\n" + block
    else:
        lines.insert(insert_at, block)
        text = "".join(lines)
    wayfire_ini.write_text(text)
    return True


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--config", type=pathlib.Path, default=None, help="keybindings TOML (default: installed/repo)")
    parser.add_argument("--wayfire-ini", type=pathlib.Path, default=pathlib.Path.home() / ".config" / "wayfire.ini")
    parser.add_argument("--apply", action="store_true", help="splice the managed block into the Wayfire config")
    args = parser.parse_args()

    config = args.config or default_config()
    if not config.exists():
        print("error: keybindings config not found: {}".format(config), file=sys.stderr)
        return 1
    block = render_block(config)
    if args.apply:
        if splice(args.wayfire_ini, block):
            print("applied agora-de keybindings to {}".format(args.wayfire_ini))
            return 0
        return 1
    sys.stdout.write(block)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
