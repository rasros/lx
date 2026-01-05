#!/usr/bin/env bash
set -euo pipefail

REPO="rasros/lx"

# Allow overriding version: VERSION=v2.0.0-rc.1 ./install.sh
target_version="${VERSION:-latest}"

# Determine install directory
# 1. Explicit override via LX_INSTALL_DIR
# 2. System-wide (/usr/local/bin) if running as root
# 3. User-local ($HOME/.local/bin) otherwise
if [ -n "${LX_INSTALL_DIR:-}" ]; then
  INSTALL_DIR="$LX_INSTALL_DIR"
elif [ "$(id -u)" -eq 0 ]; then
  INSTALL_DIR="/usr/local/bin"
else
  INSTALL_DIR="$HOME/.local/bin"
fi

log() {
  echo "[lx install] $*" >&2
}

detect_os() {
  local u
  u="$(uname -s)"
  case "$u" in
    Linux*)  echo "linux" ;;
    Darwin*) echo "darwin" ;;
    MINGW*|MSYS*|CYGWIN*) echo "windows" ;;
    *)
      log "Unsupported OS: $u"
      exit 1
      ;;
  esac
}

detect_arch() {
  local a
  a="$(uname -m)"
  case "$a" in
    x86_64|amd64)  echo "amd64" ;;
    aarch64|arm64) echo "arm64" ;;
    *)
      log "Unsupported architecture: $a"
      exit 1
      ;;
  esac
}

ensure_deps() {
  if ! command -v curl >/dev/null 2>&1; then
    log "curl is required but not found"
    exit 1
  fi
  if ! command -v grep >/dev/null 2>&1; then
    log "grep is required but not found"
    exit 1
  fi
}

ensure_unpack_tool() {
  local os="$1"
  if [ "$os" = "windows" ]; then
    if ! command -v unzip >/dev/null 2>&1; then
      log "unzip is required to extract release archives"
      exit 1
    fi
  else
    if ! command -v tar >/dev/null 2>&1; then
      log "tar is required to extract release archives"
      exit 1
    fi
  fi
}

get_release_tag() {
  if [ "$target_version" != "latest" ]; then
    # Return explicit version if requested (ensure it starts with v)
    if [[ "$target_version" != v* ]]; then
      echo "v$target_version"
    else
      echo "$target_version"
    fi
    return
  fi

  # 1. Try "latest" (Stable only)
  local api_latest="https://api.github.com/repos/${REPO}/releases/latest"
  local tag
  tag="$(curl -fsSL "$api_latest" | grep -m1 '"tag_name"' | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')"

  # 2. If no stable release (or curl failed), try "tags" to get the absolute newest (including Pre-release/RC)
  if [ -z "$tag" ]; then
    log "No stable release found, checking for pre-releases..."
    local api_tags="https://api.github.com/repos/${REPO}/tags"
    tag="$(curl -fsSL "$api_tags" | grep -m1 '"name"' | sed -E 's/.*"name": *"([^"]+)".*/\1/')"
  fi

  if [ -z "$tag" ]; then
    log "Failed to determine latest release tag"
    exit 1
  fi
  echo "$tag"
}

ensure_install_dir() {
  if [ ! -d "$INSTALL_DIR" ]; then
    log "Creating install directory: $INSTALL_DIR"
    mkdir -p "$INSTALL_DIR"
  fi
}

check_path() {
  case ":$PATH:" in
    *":$INSTALL_DIR:"*) return 0 ;;
    *)
      log "Warning: $INSTALL_DIR is not in your PATH."
      log "Add this to your shell config:"
      log "  export PATH=\"$INSTALL_DIR:\$PATH\""
      ;;
  esac
}

main() {
  ensure_deps

  os="$(detect_os)"
  arch="$(detect_arch)"
  ensure_unpack_tool "$os"

  tag="$(get_release_tag)" # e.g. "v2.0.0-rc.1"
  version="${tag#v}"       # "2.0.0-rc.1"

  log "Resolving version: $tag ($os/$arch)"

  archive_base="lx_${version}_${os}_${arch}"
  ext=".tar.gz"
  if [ "$os" = "windows" ]; then
    ext=".zip"
  fi
  archive_file="${archive_base}${ext}"

  url="https://github.com/${REPO}/releases/download/${tag}/${archive_file}"

  tmpdir="$(mktemp -d)"
  trap 'rm -rf "$tmpdir"' EXIT

  archive_path="${tmpdir}/${archive_file}"
  log "Downloading $url"
  if ! curl -fL "$url" -o "$archive_path"; then
    log "Download failed. Check if version '$tag' exists for $os/$arch."
    exit 1
  fi

  # Extract archive
  if [ "$os" = "windows" ]; then
    (cd "$tmpdir" && unzip -q "$archive_path")
  else
    (cd "$tmpdir" && tar -xzf "$archive_path")
  fi

  # Determine binary name based on release build script logic
  bin_name="lx-${version}-${os}-${arch}"
  if [ "$os" = "windows" ]; then
    bin_name="${bin_name}.exe"
  fi

  src_bin="${tmpdir}/${bin_name}"
  if [ ! -f "$src_bin" ]; then
    log "Binary not found in archive: $bin_name"
    exit 1
  fi

  ensure_install_dir
  chmod +x "$src_bin"

  # Check if we need sudo for the move
  if [ ! -w "$INSTALL_DIR" ]; then
    log "Escalating privileges to write to $INSTALL_DIR (sudo)"
    sudo mv "$src_bin" "$INSTALL_DIR/lx"
  else
    mv "$src_bin" "$INSTALL_DIR/lx"
  fi

  check_path

  log "Successfully installed lx ${version} to $INSTALL_DIR/lx"
}

main "$@"
