#!/usr/bin/env bash
set -euo pipefail

# --------------------------------------------------
# krknctl Installer
# --------------------------------------------------

BIN="krknctl"
LATEST_API="https://api.github.com/repos/krkn-chaos/krknctl/releases/latest"
BASE_URL="https://krkn-chaos.gateway.scarf.sh"

# ---------- Colors (auto disable if not TTY) ----------
if [ -t 1 ]; then
  RED="\033[0;31m"
  GREEN="\033[0;32m"
  YELLOW="\033[1;33m"
  BLUE="\033[0;34m"
  BOLD="\033[1m"
  RESET="\033[0m"
else
  RED=""; GREEN=""; YELLOW=""; BLUE=""; BOLD=""; RESET=""
fi

log()  { echo -e "${BLUE}➜${RESET} $*"; }
ok()   { echo -e "${GREEN}✓${RESET} $*"; }
warn() { echo -e "${YELLOW}!${RESET} $*"; }
err()  { echo -e "${RED}✗${RESET} $*" >&2; exit 1; }

require() {
  command -v "$1" >/dev/null 2>&1 || err "$1 is required but not installed."
}

show_banner() {
cat <<'EOF'
 _         _               _   _ 
| |       | |             | | | |
| | ___ __| | ___ __   ___| |_| |
| |/ / '__| |/ / '_ \ / __| __| |
|   <| |  |   <| | | | (__| |_| |
|_|\_\_|  |_|\_\_| |_|\___|\__|_|
                                 
EOF
}

# ---------- Requirements ----------
require curl
require tar
require install
if command -v sha256sum >/dev/null 2>&1; then
  SHA256SUM="sha256sum"
elif command -v shasum >/dev/null 2>&1; then
  SHA256SUM="shasum -a 256"
else
  err "sha256sum or shasum is required for integrity verification."
fi

# ---------- Defaults ----------
REQUESTED_VERSION=""
BINDIR=""

# ---------- Parse arguments ----------
while [ $# -gt 0 ]; do
  case "$1" in
    --version)
      REQUESTED_VERSION="${2:-}"
      [ -z "$REQUESTED_VERSION" ] && err "--version requires a value"
      shift 2
      ;;
    --bindir)
      BINDIR="${2:-}"
      [ -z "$BINDIR" ] && err "--bindir requires a value"
      shift 2
      ;;
    *)
      err "Unknown argument: $1"
      ;;
  esac
done

show_banner

# ---------- Detect OS / Arch ----------
OS="$(uname -s)"
ARCH="$(uname -m)"

case "$OS" in
  Linux)  PLATFORM="linux" ;;
  Darwin) PLATFORM="darwin" ;;
  *) err "Unsupported OS: $OS" ;;
esac

case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *) err "Unsupported architecture: $ARCH" ;;
esac

if [ "$PLATFORM" = "darwin" ]; then
  [ "$ARCH" = "arm64" ] && SUFFIX="darwin-apple-silicon" || SUFFIX="darwin-intel"
else
  SUFFIX="${PLATFORM}-${ARCH}"
fi

# ---------- Determine install directory ----------
if [ -z "$BINDIR" ]; then
  if [ -w "/usr/local/bin" ]; then
    BINDIR="/usr/local/bin"
  else
    BINDIR="$HOME/.local/bin"
  fi
fi

# ---------- Determine version ----------
if [ -n "$REQUESTED_VERSION" ]; then
  VERSION="$REQUESTED_VERSION"
  log "Using specified version: $VERSION"
else
  log "Fetching latest release..."
  VERSION="$(curl -fsSL "$LATEST_API" \
    | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')"
  [ -z "$VERSION" ] && err "Failed to fetch latest version (API rate limit or network issue)."
fi

TARBALL="${BIN}-${VERSION}-${SUFFIX}.tar.gz"
URL="${BASE_URL}/${TARBALL}"

echo
echo "${BOLD}Version:${RESET}   $VERSION"
echo "${BOLD}Platform:${RESET}  $SUFFIX"
echo "${BOLD}Install to:${RESET} $BINDIR"
echo

# ---------- Expected checksum (from GitHub release metadata) ----------
log "Fetching release checksum..."
RELEASE_JSON="$(curl -fsSL "https://api.github.com/repos/krkn-chaos/krknctl/releases/tags/${VERSION}" \
  || err "Failed to fetch release metadata from GitHub.")"
# Parse asset digest from release JSON (digest is the one following our asset name)
RELEASE_LINE="$(echo "$RELEASE_JSON" | tr -d '\n')"
REST_AFTER_NAME="$(echo "$RELEASE_LINE" | sed "s/.*\"name\": *\"${TARBALL}\"/ /")"
# Extract first 64-char hex (SHA256) from this asset block — the digest value
EXPECTED_SHA="$(echo "$REST_AFTER_NAME" | grep -oE '[0-9a-f]{64}' | head -1)"
[ "${#EXPECTED_SHA}" -eq 64 ] || err "No checksum for $TARBALL in release $VERSION. Supply-chain verification unavailable."

# ---------- Download ----------
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

log "Downloading..."
curl -fL --progress-bar "$URL" -o "$TMP/$TARBALL" \
  || err "Download failed. Check version or network."

# ---------- Verify integrity ----------
log "Verifying checksum..."
ACTUAL_SHA="$($SHA256SUM "$TMP/$TARBALL" | awk '{print $1}')"
[ "$ACTUAL_SHA" = "$EXPECTED_SHA" ] || err "Checksum verification failed (expected $EXPECTED_SHA, got $ACTUAL_SHA). The download may have been tampered with."
ok "Checksum verified"

# ---------- Extract ----------
log "Extracting..."
tar -xzf "$TMP/$TARBALL" -C "$TMP" \
  || err "Failed to extract archive."

[ -f "$TMP/$BIN" ] || err "Binary not found in archive."

mkdir -p "$BINDIR"

# ---------- Install ----------
if install -m 755 "$TMP/$BIN" "$BINDIR/$BIN" 2>/dev/null; then
  ok "Installed successfully"
else
  warn "Permission denied. Retrying with sudo..."
  sudo install -m 755 "$TMP/$BIN" "$BINDIR/$BIN" \
    || err "Installation failed."
  ok "Installed successfully"
fi

# ---------- Summary ----------
echo
echo "----------------------------------------"
echo " Binary   : $BINDIR/$BIN"
echo " Version  : $VERSION"
echo " Platform : $SUFFIX"
echo "----------------------------------------"
echo

# ---------- PATH Check ----------
case ":$PATH:" in
  *":$BINDIR:"*) ;;
  *)
    warn "$BINDIR is not in your PATH."
    echo "Add it with:"
    echo "  export PATH=\"$BINDIR:\$PATH\""
    echo
    ;;
esac

# ---------- Verification ----------
if command -v "$BIN" >/dev/null 2>&1; then
  INSTALLED_VERSION="$($BIN version 2>/dev/null || true)"
  ok "Installation verified"
  [ -n "$INSTALLED_VERSION" ] && echo "Detected: $INSTALLED_VERSION"
fi

echo
ok "krknctl installation complete!"
echo "Run: $BIN --help"
echo
