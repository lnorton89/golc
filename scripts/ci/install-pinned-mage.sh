#!/usr/bin/env bash
# Installs the checksum-pinned Go and Mage archives from
# config/toolchain.toml for the running platform (Linux/macOS), verifies
# each SHA-256 against its exact committed pin, extracts it, and adds its
# binary directory to $GITHUB_PATH so subsequent workflow steps can
# invoke "go"/"mage" directly.
#
# Both are installed, not just Mage: an ambient `go install
# github.com/magefile/mage@...` cannot be relied on in CI (absent
# entirely on some hosted macOS runners, and even where it succeeds its
# GOPATH/bin output is not on PATH for later steps by default) -- and
# Mage itself always needs *some* Go compiler on PATH regardless, because
# it JIT-compiles the magefile package at every invocation rather than
# shipping a precompiled runner. Installing the project's own pinned Go
# here, the same way every other route in this project already refuses
# to trust an ambient toolchain (resolvePinnedGoExecutable never does a
# host PATH lookup either), means Mage's JIT compile never depends on
# whatever Go version (if any) a given hosted runner image happens to
# ship.
#
# This script reads config/toolchain.toml directly rather than duplicating
# its committed URL/checksum values, so it can never drift from the single
# authoritative pin (D-05/D-09: refer, never repeat).
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
toolchain_file="${repo_root}/config/toolchain.toml"

case "$(uname -s)" in
  Linux) os="linux" ;;
  Darwin) os="darwin" ;;
  *) echo "install-pinned-mage.sh: unsupported OS $(uname -s)" >&2; exit 1 ;;
esac
case "$(uname -m)" in
  x86_64|amd64) arch="amd64" ;;
  arm64|aarch64) arch="arm64" ;;
  *) echo "install-pinned-mage.sh: unsupported architecture $(uname -m)" >&2; exit 1 ;;
esac
platform="${os}-${arch}"

read_pin() {
  # $1 = toolchain name (e.g. "go", "mage"), $2 = key (archive_url/archive_sha256)
  local section="[toolchain.${1}.platforms.\"${platform}\"]"
  awk -v section="$section" -v key="$2" '
    $0 == section { found = 1; next }
    found && /^\[/ { found = 0 }
    found && $1 == key {
      value = $0
      sub(/^[^"]*"/, "", value)
      sub(/".*$/, "", value)
      print value
      exit
    }
  ' "$toolchain_file"
}

sha256_of() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  else
    shasum -a 256 "$1" | awk '{print $1}'
  fi
}

# install_pinned_archive installs one checksum-pinned tool and echoes the
# extraction directory it was unpacked into (the caller locates the exact
# binary itself, since Go's and Mage's archives place it at different
# relative paths).
install_pinned_archive() {
  local tool="$1"
  local archive_url archive_sha256
  archive_url=$(read_pin "$tool" "archive_url")
  archive_sha256=$(read_pin "$tool" "archive_sha256")
  if [ -z "$archive_url" ] || [ -z "$archive_sha256" ]; then
    echo "install-pinned-mage.sh: no committed ${tool} pin for platform ${platform} in ${toolchain_file}" >&2
    exit 1
  fi

  local staging archive_path
  staging="$(mktemp -d)"
  archive_path="${staging}/${tool}-archive"
  # Only the downloaded archive is cleaned up; the extraction directory
  # this function echoes must survive so the caller's binary keeps
  # existing on disk for the rest of the job. archive_path is still a
  # valid local variable when this trap fires at function return.
  trap 'rm -f "$archive_path"' RETURN
  curl -sSL --fail -o "$archive_path" "$archive_url"

  local actual
  actual=$(sha256_of "$archive_path")
  if [ "$actual" != "$archive_sha256" ]; then
    echo "install-pinned-mage.sh: checksum mismatch for ${archive_url}: got ${actual}, want ${archive_sha256}" >&2
    exit 1
  fi

  local extract_dir="${staging}/extracted"
  mkdir -p "$extract_dir"
  tar -xzf "$archive_path" -C "$extract_dir"
  echo "$extract_dir"
}

go_extract_dir=$(install_pinned_archive "go")
go_binary="${go_extract_dir}/go/bin/go"
if [ ! -f "$go_binary" ]; then
  echo "install-pinned-mage.sh: extracted Go archive does not contain go/bin/go" >&2
  exit 1
fi
chmod +x "$go_binary"
dirname "$go_binary" >> "$GITHUB_PATH"
echo "install-pinned-mage.sh: installed checksum-verified Go ${platform}"

mage_extract_dir=$(install_pinned_archive "mage")
mage_binary=$(find "$mage_extract_dir" -type f -name 'mage' -print -quit)
if [ -z "$mage_binary" ]; then
  echo "install-pinned-mage.sh: extracted Mage archive does not contain a mage binary" >&2
  exit 1
fi
chmod +x "$mage_binary"
dirname "$mage_binary" >> "$GITHUB_PATH"
echo "install-pinned-mage.sh: installed checksum-verified Mage ${platform}"
