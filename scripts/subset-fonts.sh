#!/usr/bin/env bash
# Rewrites the embedded CJK fonts into subsets.
#
# Strikeye embeds Noto Sans SC (OFL) in all three binaries, and the full font is
# 10.5 MB per weight. Subsetting to GBK plus every non-ASCII rune the Go sources
# can print keeps all UI text and ordinary Chinese names intact at 7.7 MB.
#
# Usage: scripts/subset-fonts.sh /path/to/full/NotoSansSC-Medium.ttf [/path/to/Bold]
# Requires fonttools (pip install fonttools).
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
if [[ $# -lt 1 ]]; then
  echo "usage: $0 <NotoSansSC-Medium.ttf> [<NotoSansSC-Bold.ttf>]" >&2
  exit 2
fi
if ! command -v pyftsubset >/dev/null; then
  echo "pyftsubset not found: pip install fonttools" >&2
  exit 2
fi

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

python3 - "$repo_root" "$work/charset.txt" <<'PY'
import glob, os, sys

repo, out = sys.argv[1], sys.argv[2]
chars = set(chr(c) for c in range(0x20, 0x7F))
chars |= set(chr(c) for c in range(0xA0, 0x100))
chars |= set(chr(c) for c in range(0x2000, 0x2070))
chars |= set(chr(c) for c in range(0x3000, 0x3040))
chars |= set(chr(c) for c in range(0xFF01, 0xFF61))
for hi in range(0x81, 0xFF):
    for lo in range(0x40, 0xFF):
        try:
            chars.add(bytes([hi, lo]).decode("gbk"))
        except UnicodeDecodeError:
            pass
for path in glob.glob(os.path.join(repo, "internal", "**", "*.go"), recursive=True) + \
            glob.glob(os.path.join(repo, "cmd", "**", "*.go"), recursive=True):
    chars |= set(open(path, encoding="utf-8").read())
chars = {c for c in chars if (c.isprintable() and ord(c) > 31) or c == " "}
with open(out, "w", encoding="utf-8") as fh:
    fh.write("".join(sorted(chars)))
PY

subset() {
  local src="$1" dest="$2"
  pyftsubset "$src" --text-file="$work/charset.txt" --layout-features='*' \
    --output-file="$work/$(basename "$dest")"
  mv "$work/$(basename "$dest")" "$dest"
  printf '%s: %s bytes\n' "$dest" "$(wc -c <"$dest")"
}

subset "$1" "$repo_root/assets/fonts/NotoSansSC-Medium.ttf"
if [[ $# -ge 2 ]]; then
  subset "$2" "$repo_root/assets/fonts/NotoSansSC-Bold.ttf"
fi
