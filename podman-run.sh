#!/usr/bin/env bash

set -euo pipefail

IMAGE_NAME="chaos-dashboard:latest"
CONTAINER_NAME="chaos-dashboard-app"
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CHAOS_ASSETS="$ROOT_DIR/src/assets"

# krkn-hub images are commonly amd64-only; default to amd64 for cross-machine compatibility.
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64) PODMAN_PLATFORM="linux/amd64" ;;
  aarch64|arm64) PODMAN_PLATFORM="linux/amd64" ;;
  *) PODMAN_PLATFORM="linux/amd64" ;;
esac

BUILD_PLATFORM_ARGS=(--platform "$PODMAN_PLATFORM")
RUN_PLATFORM_ARGS=(--platform "$PODMAN_PLATFORM")
ENV_PLATFORM_ARGS=(-e "PODMAN_PLATFORM=$PODMAN_PLATFORM")
GROUP_ARGS=()

# Sockets may be /run/podman/podman.sock (rootful / some VMs) or
# /run/user/$UID/podman/podman.sock (rootless). On macOS/Windows, discovery runs in the VM via
# `podman machine ssh NAME '...'`. You must pass NAME (first arg); if you omit it, `sh` is
# treated as the machine name. Pass one remote command in single host quotes — do not use
# `sh -c '...'` here: Podman 5 forwards that badly (remote sees `sh -c for ...` and errors).
PODMAN_MACHINE="${PODMAN_MACHINE:-$(podman machine list --format '{{range .}}{{if .Running}}{{.Name}}{{"\n"}}{{end}}{{end}}' 2>/dev/null | head -n1)}"
PODMAN_MACHINE="${PODMAN_MACHINE//\*/}"

VM_SOCK_PATH=""
SOCKET_IN_MACHINE=false
if [ -n "$PODMAN_MACHINE" ] && VM_SOCK_PATH="$(podman machine ssh "$PODMAN_MACHINE" 'for p in /run/podman/podman.sock /run/user/$(id -u)/podman/podman.sock; do [ -S "$p" ] && printf %s "$p" && exit 0; done; f=$(find /run /var/run -type s -name podman.sock 2>/dev/null | head -n1); [ -n "$f" ] && [ -S "$f" ] && printf %s "$f" && exit 0; exit 1' 2>/dev/null)"; then
  SOCKET_IN_MACHINE=true
else
  for p in /run/podman/podman.sock "/run/user/$(id -u)/podman/podman.sock"; do
    if [ -S "$p" ]; then
      VM_SOCK_PATH="$p"
      break
    fi
  done
fi

if [ -z "$VM_SOCK_PATH" ]; then
  echo "[error] Could not find podman.sock (machine: ${PODMAN_MACHINE:-none}; also tried /run/... on the host)." >&2
  echo "        Start podman (or \`podman machine start\`) and ensure the socket exists." >&2
  exit 1
fi

SOCK_GID=""
if [ "$SOCKET_IN_MACHINE" = true ] && [ -n "$PODMAN_MACHINE" ]; then
  SOCK_GID="$(podman machine ssh "$PODMAN_MACHINE" stat -c %g "$VM_SOCK_PATH" 2>/dev/null)" || true
else
  SOCK_GID="$(stat -c %g "$VM_SOCK_PATH" 2>/dev/null)" || true
fi
if [ -n "$SOCK_GID" ]; then
  GROUP_ARGS=(--group-add "$SOCK_GID")
else
  echo "[warn] Could not read socket group id; continuing without --group-add." >&2
  echo "[warn] If you hit EACCES on the socket, fix permissions or use \`podman machine ssh\` to inspect the socket." >&2
fi

podman rm -f "$CONTAINER_NAME" 2>/dev/null || true
podman build "${BUILD_PLATFORM_ARGS[@]}" -t "$IMAGE_NAME" -f "$ROOT_DIR/containers/Dockerfile" "$ROOT_DIR"

podman run -d \
  "${RUN_PLATFORM_ARGS[@]}" \
  "${ENV_PLATFORM_ARGS[@]}" \
  ${GROUP_ARGS[@]+"${GROUP_ARGS[@]}"} \
  -e "CHAOS_ASSETS=$CHAOS_ASSETS" \
  --security-opt label=disable \
  -v "$CHAOS_ASSETS:/usr/src/chaos-dashboard/src/assets:z" \
  -v "$ROOT_DIR/database:/usr/src/chaos-dashboard/database:z" \
  -v "$VM_SOCK_PATH:/run/podman/podman.sock:z" \
  -p 3000:3000 \
  -p 8000:8000 \
  --name "$CONTAINER_NAME" \
  "$IMAGE_NAME"

echo "Started $CONTAINER_NAME (open http://localhost:3000)"
