#!/usr/bin/env bash

set -euo pipefail

repo="xrmcp/cli"
project="xrmcp"

version_tag=""
install_dir=""

usage() {
  cat <<'EOF'
Usage: install.sh [--version vX.Y.Z] [--install-dir DIR]

Installs xrmcp from GitHub Releases on Linux.
EOF
}

log() {
  printf '%s\n' "$*"
}

fail() {
  printf 'error: %s\n' "$*" >&2
  exit 1
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || fail "required command not found: $1"
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --version)
      [ "$#" -ge 2 ] || fail "--version requires a value"
      version_tag="$2"
      shift 2
      ;;
    --install-dir)
      [ "$#" -ge 2 ] || fail "--install-dir requires a value"
      install_dir="$2"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      fail "unknown argument: $1"
      ;;
  esac
done

[ "$(uname -s)" = "Linux" ] || fail "this installer supports Linux only"

require_cmd curl
require_cmd tar

arch="$(uname -m)"
case "$arch" in
  x86_64|amd64)
    arch="amd64"
    ;;
  aarch64|arm64)
    arch="arm64"
    ;;
  *)
    fail "unsupported architecture: $arch"
    ;;
esac

if [ -z "$version_tag" ]; then
  latest_url="https://github.com/${repo}/releases/latest"
  version_tag="$(curl -fsSL -o /dev/null -w '%{url_effective}' "$latest_url")"
  version_tag="${version_tag##*/}"
fi

case "$version_tag" in
  v*) ;;
  *)
    version_tag="v${version_tag}"
    ;;
esac

version="${version_tag#v}"

if [ -z "$install_dir" ]; then
  if [ -w "/usr/local/bin" ]; then
    install_dir="/usr/local/bin"
  else
    install_dir="${HOME}/.local/bin"
  fi
fi

archive="${project}_${version}_linux_${arch}.tar.gz"
checksums="sha256sums.txt"
base_url="https://github.com/${repo}/releases/download/${version_tag}"
archive_url="${base_url}/${archive}"
checksums_url="${base_url}/${checksums}"

tmpdir="$(mktemp -d)"
cleanup() {
  rm -rf "$tmpdir"
}
trap cleanup EXIT

archive_path="${tmpdir}/${archive}"
checksums_path="${tmpdir}/${checksums}"
extract_dir="${tmpdir}/extract"

log "Installing ${project} ${version_tag} for linux/${arch}"
log "Downloading ${archive_url}"
curl -fsSL "$archive_url" -o "$archive_path"

if curl -fsSL "$checksums_url" -o "$checksums_path"; then
  checksum_line="$(grep " ${archive}\$" "$checksums_path" || true)"
  if [ -n "$checksum_line" ]; then
    if command -v sha256sum >/dev/null 2>&1; then
      printf '%s\n' "$checksum_line" | (cd "$tmpdir" && sha256sum -c -)
    elif command -v shasum >/dev/null 2>&1; then
      expected_sum="$(printf '%s\n' "$checksum_line" | awk '{print $1}')"
      actual_sum="$(shasum -a 256 "$archive_path" | awk '{print $1}')"
      [ "$expected_sum" = "$actual_sum" ] || fail "checksum verification failed"
    else
      log "warning: sha256sum/shasum not found; skipping checksum verification"
    fi
  else
    log "warning: checksum entry not found for ${archive}; skipping verification"
  fi
else
  log "warning: could not download ${checksums}; skipping verification"
fi

mkdir -p "$extract_dir"
tar -xzf "$archive_path" -C "$extract_dir"

binary_path="$(find "$extract_dir" -type f -name xrmcp | head -n 1)"
[ -n "$binary_path" ] || fail "could not find xrmcp binary in archive"

mkdir -p "$install_dir"
install -m 0755 "$binary_path" "${install_dir}/xrmcp"

log "Installed to ${install_dir}/xrmcp"

case ":${PATH}:" in
  *":${install_dir}:"*) ;;
  *)
    log "Add ${install_dir} to PATH if it is not already available in your shell."
    ;;
esac

log "Run 'xrmcp --help' to verify the installation."
