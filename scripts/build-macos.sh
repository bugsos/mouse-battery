#!/bin/sh
set -eu
cd "$(dirname "$0")/.."

app="bin/Mouse Battery.app"
mkdir -p "$app/Contents/MacOS" "$app/Contents/Resources"
cp scripts/Info.plist "$app/Contents/Info.plist"

if [ -f assets/icon.png ] && command -v sips >/dev/null 2>&1 && command -v iconutil >/dev/null 2>&1; then
  iconset="$(mktemp -d)/AppIcon.iconset"
  mkdir -p "$iconset"
  for sz in 16 32 128 256 512; do
    sips -z "$sz" "$sz" assets/icon.png --out "$iconset/icon_${sz}x${sz}.png" >/dev/null
    sz2=$((sz * 2))
    if [ "$sz2" -le 512 ]; then
      sips -z "$sz2" "$sz2" assets/icon.png --out "$iconset/icon_${sz}x${sz}@2x.png" >/dev/null
    fi
  done
  iconutil -c icns "$iconset" -o "$app/Contents/Resources/AppIcon.icns"
  rm -rf "$(dirname "$iconset")"
fi

CGO_ENABLED=1 go build -trimpath -buildvcs=false -ldflags="-s -w" -o "$app/Contents/MacOS/mouse-battery" .
codesign --force --sign - "$app"
printf '%s\n' "$app"
