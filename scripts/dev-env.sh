# Source this in a POSIX shell (Git Bash on Windows) before running Go, bun lint/test/codegen,
# or the media tests:
#
#   source scripts/dev-env.sh
#
# It fixes two Windows-only problems and does nothing on a machine that does not have them
# (docs/HANDOVER.md §3):
#   1. Go: the installed Go is older than go.mod's, so Go auto-downloads 1.26 and then fails with
#      `compile: version "go1.26.0" does not match go tool version`. The fix is to put the cached
#      1.26 toolchain first on PATH and stop Go from switching.
#   2. ffmpeg, ffprobe, vips and vipsheader (installed with winget) only reach the PATH of
#      terminals opened after the install, so older shells do not see them.
# No `set -e`: this file is sourced and must not end your shell.

_tepati_prepend() { # prepend a directory to PATH once
  case ":$PATH:" in *":$1:"*) ;; *) PATH="$1:$PATH" ;; esac
}

for _d in "$HOME"/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26*.windows-amd64/bin; do
  if [ -d "$_d" ]; then
    _tepati_prepend "$_d"
    export GOTOOLCHAIN=local
  fi
done

_winget="${LOCALAPPDATA:-$HOME/AppData/Local}/Microsoft/WinGet/Packages"
for _d in "$_winget"/Gyan.FFmpeg_*/ffmpeg-*/bin "$_winget"/libvips.libvips_*/vips-dev-*/bin; do
  [ -d "$_d" ] && _tepati_prepend "$_d"
done
export PATH

_tepati_report() {
  if command -v "$1" >/dev/null 2>&1; then
    printf '  %-10s %s\n' "$1" "$("$@" 2>&1 | head -1)"
  else
    printf '  %-10s MISSING\n' "$1"
  fi
}
echo "dev environment:"
_tepati_report go version
_tepati_report ffmpeg -version
_tepati_report ffprobe -version
_tepati_report vips --version
_tepati_report vipsheader --version

unset _d _winget
unset -f _tepati_prepend _tepati_report
