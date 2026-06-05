#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

if [[ "$(uname -s)" != "Darwin" ]]; then
  echo "macOS packaging must run on macOS. Wails v2 cannot cross-compile macOS apps from Windows/Linux." >&2
  exit 1
fi

if ! command -v go >/dev/null 2>&1; then
  echo "Go is required. Install Go 1.23+ first." >&2
  exit 1
fi

if ! command -v pnpm >/dev/null 2>&1; then
  echo "pnpm is required. Install it with: npm install -g pnpm" >&2
  exit 1
fi

if ! command -v wails >/dev/null 2>&1; then
  echo "Wails CLI not found, installing v2.12.0..."
  go install github.com/wailsapp/wails/v2/cmd/wails@v2.12.0
  export PATH="$(go env GOPATH)/bin:$PATH"
fi

APP_NAME="FusionGitDesk"
DIST_DIR="$ROOT_DIR/build/dist"
APP_PATH="$ROOT_DIR/build/bin/${APP_NAME}.app"

rm -rf "$DIST_DIR"
mkdir -p "$DIST_DIR"

wails build -clean -platform darwin/universal

if [[ ! -d "$APP_PATH" ]]; then
  echo "Expected app bundle was not generated: $APP_PATH" >&2
  exit 1
fi

# Ad-hoc signing avoids some local macOS launch friction without requiring an Apple Developer certificate.
codesign --force --deep --sign - "$APP_PATH"

ditto -c -k --keepParent "$APP_PATH" "$DIST_DIR/${APP_NAME}-macos-universal.app.zip"
hdiutil create \
  -volname "Fusion Git Desk" \
  -srcfolder "$APP_PATH" \
  -ov \
  -format UDZO \
  "$DIST_DIR/${APP_NAME}-macos-universal.dmg"

echo "macOS packages generated:"
ls -lh "$DIST_DIR"
