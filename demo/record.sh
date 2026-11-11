#!/bin/sh
# Records the demo of the UI luasrcpum into demo/out/ (a .cast, a .gif and an
# .mp4). Needs nvim, go, gopls, git, asciinema, agg and ffmpeg, and the Go
# parser of treesitter (see demo/init.lua).
#
#   GODA_DEMO_PLUGINS  directory of goda.vim, goda-cmp.vim, pum.vim and
#                      colorful-menu.nvim (cloned there when missing)
#   AGG_FONT           font family for agg
set -eu
for c in nvim go gopls git asciinema agg ffmpeg; do
	command -v "$c" >/dev/null || { echo "$c is not in PATH" >&2; exit 1; }
done
cd "$(dirname "$0")/.."
dir=${GODA_DEMO_DIR:-${XDG_CACHE_HOME:-$HOME/.cache}/goda-cmp-ext-luasrc-demo}
plugins=${GODA_DEMO_PLUGINS:-$dir/plugins}
mkdir -p "$plugins" "$dir/cache" demo/out
for repo in Shougo/goda.vim Shougo/goda-cmp.vim Shougo/pum.vim xzbdmw/colorful-menu.nvim; do
	d=$plugins/${repo#*/}
	[ -d "$d" ] || git clone -q --depth 1 "https://github.com/$repo" "$d"
done
export GODA_DEMO_PLUGINS="$plugins" GODA_DEMO_CACHE="$dir/cache"

# Build outside the recording.
rm -f "$dir/cache/goda-cmp"
nvim --headless -u demo/init.lua -i NONE -c 'call goda_cmp#build()' -c qa

GODA_DEMO_DRIVE=1 asciinema rec --overwrite -q --window-size 120x18 \
	-c "nvim -u demo/init.lua -i NONE demo/sample/main.go" demo/out/luasrcpum.cast
# The demo file is typed into; the recording quits without writing it. Drop
# the quitting (the screen cleared) from the end.
python3 - demo/out/luasrcpum.cast <<'PY2'
import json, sys
lines = open(sys.argv[1]).read().splitlines()
head, events = lines[0], [json.loads(l) for l in lines[1:]]
for i, e in enumerate(events):
    if e[1] == "o" and "\x1b[?1049l" in e[2]:
        events = events[:i]
        break
open(sys.argv[1], "w").write("\n".join([head] + [json.dumps(e) for e in events]) + "\n")
PY2
agg ${AGG_FONT:+--font-family "$AGG_FONT"} --font-size 20 --last-frame-duration 2 \
	demo/out/luasrcpum.cast demo/out/luasrcpum.gif
ffmpeg -y -loglevel error -i demo/out/luasrcpum.gif -movflags +faststart -pix_fmt yuv420p \
	-vf 'scale=trunc(iw/2)*2:trunc(ih/2)*2' demo/out/luasrcpum.mp4
ls -l demo/out
