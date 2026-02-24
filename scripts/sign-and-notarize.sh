#!/usr/bin/env bash
# sign-and-notarize.sh — Signs and notarizes a macOS binary using Apple Developer ID.
#
# Called by GoReleaser as a build post-hook for each target. Skips automatically
# when not running on macOS or when the target OS is not darwin.
#
# Required environment variables:
#   APPLE_DEVELOPER_IDENTITY  Full cert name, e.g. "Developer ID Application: Acme Inc (ABC123DEF4)"
#   NOTARIZE_ISSUER_ID        App Store Connect API Issuer ID (UUID)
#   NOTARIZE_KEY_ID           App Store Connect API Key ID (10-char)
#   NOTARIZE_KEY_PATH         Absolute path to the downloaded .p8 API key file
#
# Usage (invoked by GoReleaser):
#   ./scripts/sign-and-notarize.sh <binary-path> <goos>

set -euo pipefail

BINARY="${1:?Usage: $0 <binary-path> <goos>}"
TARGET_OS="${2:?Usage: $0 <binary-path> <goos>}"
BUNDLE_ID="ca.keyharbour.kh"

# Only run on a macOS host — cross-compiled Linux/Windows binaries are skipped here.
if [[ "$(uname -s)" != "Darwin" ]]; then
  echo "  → Skipping code signing: not running on macOS"
  exit 0
fi

if [[ "${TARGET_OS}" != "darwin" ]]; then
  echo "  → Skipping code signing: target OS is ${TARGET_OS}"
  exit 0
fi

: "${APPLE_DEVELOPER_IDENTITY:?APPLE_DEVELOPER_IDENTITY must be set}"
: "${NOTARIZE_ISSUER_ID:?NOTARIZE_ISSUER_ID must be set}"
: "${NOTARIZE_KEY_ID:?NOTARIZE_KEY_ID must be set}"
: "${NOTARIZE_KEY_PATH:?NOTARIZE_KEY_PATH must be set}"

echo "  → Signing: ${BINARY}"
codesign \
  --force \
  --options runtime \
  --sign "${APPLE_DEVELOPER_IDENTITY}" \
  --identifier "${BUNDLE_ID}" \
  "${BINARY}"

codesign --verify --verbose=4 "${BINARY}"
echo "  → Signature verified"

# Notarize the binary.
# Note: xcrun stapler staple only works for .app/.pkg/.dmg bundles, not plain
# executables. For CLI binaries, notarization is sufficient — Gatekeeper checks
# Apple's online notarization database when the quarantine bit is set.
NOTARIZE_ZIP="$(mktemp -t kh-notarize).zip"
ditto -c -k --keepParent "${BINARY}" "${NOTARIZE_ZIP}"

echo "  → Submitting for notarization (may take a few minutes)…"
xcrun notarytool submit "${NOTARIZE_ZIP}" \
  --issuer "${NOTARIZE_ISSUER_ID}" \
  --key-id "${NOTARIZE_KEY_ID}" \
  --key "${NOTARIZE_KEY_PATH}" \
  --wait \
  --timeout 10m

rm -f "${NOTARIZE_ZIP}"
echo "  → Notarization complete: ${BINARY}"
