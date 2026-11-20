#!/bin/sh
set -eu
cd "$(dirname "$0")/.."

app="bin/Mouse Battery.app"
mkdir -p "$app/Contents/MacOS"
cp scripts/Info.plist "$app/Contents/Info.plist"
CGO_ENABLED=1 go build -trimpath -ldflags="-s -w" -o "$app/Contents/MacOS/mouse-battery" .
codesign --force --sign - "$app"
printf '%s\n' "$app"
