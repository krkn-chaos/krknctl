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
  API_RESPONSE="$(curl -fsSL "$LATEST_API" 2>/dev/null)" || true
  [ -z "$API_RESPONSE" ] && err "Failed to fetch latest version (API rate limit or network issue)."
  if command -v jq >/dev/null 2>&1; then
    VERSION="$(echo "$API_RESPONSE" | jq -r '.tag_name // empty')"
  fi
  if [ -z "$VERSION" ]; then
    VERSION="$(echo "$API_RESPONSE" | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')"
  fi
  [ -z "$VERSION" ] && err "Failed to parse latest version from API response."
fi

TARBALL="${BIN}-${VERSION}-${SUFFIX}.tar.gz"
URL="${BASE_URL}/${TARBALL}"

printf '\n'
printf '%b\n' "${BOLD}Version:${RESET}   $VERSION"
printf '%b\n' "${BOLD}Platform:${RESET}  $SUFFIX"
printf '%b\n' "${BOLD}Install to:${RESET} $BINDIR"
printf '\n'

# ---------- Expected checksum (from GitHub release metadata) ----------
log "Fetching release checksum..."
RELEASE_JSON="$(curl -fsSL "https://api.github.com/repos/krkn-chaos/krknctl/releases/tags/${VERSION}" 2>&1)" || \
  err "Failed to fetch release metadata from GitHub. curl output: $RELEASE_JSON"
EXPECTED_SHA=""
if command -v jq >/dev/null 2>&1; then
  EXPECTED_SHA="$(echo "$RELEASE_JSON" | jq -r --arg name "$TARBALL" '.assets[] | select(.name == $name) | .digest | sub("sha256:"; "")')"
fi
if [ -z "$EXPECTED_SHA" ] || [ "${#EXPECTED_SHA}" -ne 64 ]; then
  # Fallback: parse asset digest without jq (fragile if API format changes)
  RELEASE_LINE="$(echo "$RELEASE_JSON" | tr -d '\n')"
  REST_AFTER_NAME="$(echo "$RELEASE_LINE" | sed "s/.*\"name\": *\"${TARBALL}\"/ /")"
  EXPECTED_SHA="$(echo "$REST_AFTER_NAME" | grep -oE '[0-9a-f]{64}' | head -1)"
fi
[ "${#EXPECTED_SHA}" -eq 64 ] || err "No checksum for $TARBALL in release $VERSION. Supply-chain verification unavailable."

# ---------- Download ----------
TMP="$(mktemp -d 2>/dev/null)" || TMP="$(mktemp -d -t krknctl 2>/dev/null)" || true
[ -n "${TMP:-}" ] && [ -d "$TMP" ] || err "Failed to create temporary directory (mktemp failed)."
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
[ -x "$TMP/$BIN" ] || err "Extracted file is not executable."
[ -x "$TMP/$BIN" ] || err "Extracted file is not executable."

mkdir -p "$BINDIR"

# ---------- Install ----------
if [ -w "$BINDIR" ]; then
  install -m 755 "$TMP/$BIN" "$BINDIR/$BIN" || err "Installation failed. Check path and disk space."
  ok "Installed successfully"
else
  warn "No write permission for $BINDIR. Using sudo..."
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

# ---------- Verification (use $BINDIR/$BIN so we verify what we just installed) ----------
INSTALLED_VERSION=""
if [ -x "$BINDIR/$BIN" ]; then
  if INSTALLED_VERSION="$("$BINDIR/$BIN" --version 2>/dev/null)"; then
    :
  elif INSTALLED_VERSION="$("$BINDIR/$BIN" version 2>/dev/null)"; then
    :
  fi
  if [ -n "$INSTALLED_VERSION" ]; then
    ok "Installation verified"
    echo "$INSTALLED_VERSION" | head -1
  else
    warn "Installation complete but could not verify version ($BINDIR/$BIN --version failed)."
  fi
  # Warn if another krknctl earlier in PATH would shadow this install
  RESOLVED="$(command -v "$BIN" 2>/dev/null)" || true
  if [ -n "$RESOLVED" ]; then
    NORM_INSTALLED="$(cd "$BINDIR" 2>/dev/null && pwd -P)/$BIN"
    NORM_RESOLVED="$(cd "$(dirname "$RESOLVED")" 2>/dev/null && pwd -P)/$(basename "$RESOLVED")"
    [ "$NORM_INSTALLED" != "$NORM_RESOLVED" ] && warn "Another $BIN at $RESOLVED is earlier in PATH; \"$BIN\" will run that one, not $BINDIR/$BIN."
  fi
fi

echo
ok "krknctl installation complete!"
echo "Run: $BIN --help"
echo
