#!/usr/bin/env sh
set -eu

repo="NomiciAI/nomici-orchestrator"
install_dir="${HOME}/.local/bin"
version="latest"
from_source=""
skip_checksum="false"
uninstall="false"

usage() {
  cat <<'USAGE'
Nomici Orchestrator installer

Usage:
  install.sh [--version <tag>] [--install-dir <dir>] [--from-source <dir>] [--skip-checksum]
  install.sh --uninstall [--install-dir <dir>]

Defaults:
  --install-dir ~/.local/bin
  --version latest

The installer does not use sudo by default and does not modify Nomici config.
USAGE
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --version)
      version="${2:?missing --version value}"
      shift 2
      ;;
    --install-dir)
      install_dir="${2:?missing --install-dir value}"
      shift 2
      ;;
    --from-source)
      from_source="${2:?missing --from-source value}"
      shift 2
      ;;
    --skip-checksum)
      skip_checksum="true"
      shift
      ;;
    --uninstall)
      uninstall="true"
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

target="${install_dir}/nomici"

if [ "$uninstall" = "true" ]; then
  if [ -e "$target" ]; then
    rm -f "$target"
    echo "Removed $target"
  else
    echo "Nomici is not installed at $target"
  fi
  exit 0
fi

mkdir -p "$install_dir"

backup_existing() {
  if [ -e "$target" ]; then
    backup="${target}.bak.$(date +%Y%m%d%H%M%S)"
    mv "$target" "$backup"
    echo "Backed up existing nomici to $backup"
  fi
}

install_binary() {
  src="$1"
  backup_existing
  cp "$src" "$target"
  chmod 0755 "$target"
  echo "Installed nomici to $target"
  if ! command -v nomici >/dev/null 2>&1; then
    echo "Add $install_dir to PATH if nomici is not found by your shell."
  fi
}

install_from_source() {
  src_dir="$1"
  if [ ! -f "$src_dir/go.mod" ] || [ ! -d "$src_dir/cmd/nomici" ]; then
    echo "--from-source must point to the nomici-orchestrator repository root" >&2
    exit 1
  fi
  if ! command -v make >/dev/null 2>&1; then
    echo "make is required for source install" >&2
    exit 1
  fi
  (
    cd "$src_dir"
    VERSION="$version" make build
  )
  install_binary "$src_dir/bin/nomici"
}

detect_platform() {
  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  arch="$(uname -m)"
  case "$os" in
    darwin|linux) ;;
    *) echo "Unsupported OS: $os" >&2; exit 1 ;;
  esac
  case "$arch" in
    x86_64|amd64) arch="amd64" ;;
    arm64|aarch64) arch="arm64" ;;
    *) echo "Unsupported architecture: $arch" >&2; exit 1 ;;
  esac
  echo "${os}_${arch}"
}

sha256_cmd() {
  if command -v sha256sum >/dev/null 2>&1; then
    echo "sha256sum"
  elif command -v shasum >/dev/null 2>&1; then
    echo "shasum -a 256"
  else
    echo ""
  fi
}

download_release() {
  if ! command -v curl >/dev/null 2>&1; then
    echo "curl is required for release install" >&2
    exit 1
  fi
  platform="$(detect_platform)"
  tmp="$(mktemp -d)"
  artifact="nomici_${platform}.tar.gz"
  if [ "$version" = "latest" ]; then
    base="https://github.com/${repo}/releases/latest/download"
  else
    base="https://github.com/${repo}/releases/download/${version}"
  fi

  echo "Downloading ${artifact} from ${base}"
  curl -fsSL "${base}/${artifact}" -o "${tmp}/${artifact}"

  if [ "$skip_checksum" != "true" ]; then
    checksum_tool="$(sha256_cmd)"
    if [ -z "$checksum_tool" ]; then
      echo "No SHA256 tool found. Install sha256sum or shasum, or rerun with --skip-checksum." >&2
      exit 1
    fi
    curl -fsSL "${base}/checksums.txt" -o "${tmp}/checksums.txt"
    expected="$(grep " ${artifact}\$" "${tmp}/checksums.txt" | awk '{print $1}')"
    if [ -z "$expected" ]; then
      echo "checksums.txt does not contain ${artifact}" >&2
      exit 1
    fi
    actual="$($checksum_tool "${tmp}/${artifact}" | awk '{print $1}')"
    if [ "$actual" != "$expected" ]; then
      echo "SHA256 mismatch for ${artifact}" >&2
      exit 1
    fi
  fi

  tar -xzf "${tmp}/${artifact}" -C "$tmp"
  if [ ! -x "${tmp}/nomici" ]; then
    echo "Release archive did not contain executable nomici" >&2
    exit 1
  fi
  install_binary "${tmp}/nomici"
}

if [ -n "$from_source" ]; then
  install_from_source "$from_source"
else
  download_release
fi

"$target" --version
