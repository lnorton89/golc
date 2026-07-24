#!/usr/bin/env bash
# Installs the checksum-pinned Mage archive from config/toolchain.toml for
# the running platform (Linux/macOS), verifies its SHA-256 against the
# exact committed pin, extracts it, and adds its directory to
# $GITHUB_PATH so subsequent workflow steps can invoke "mage" directly.
#
# This exists because an ambient `go install github.com/magefile/mage@...`
# cannot be relied on in CI: it is absent entirely on some hosted macOS
# runners (no ambient Go on PATH), and even where it succeeds, its
# GOPATH/bin output is not on PATH for later steps by default.
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
section="[toolchain.mage.platforms.\"${platform}\"]"

read_pin() {
  awk -v section="$section" -v key="$1" '
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

archive_url=$(read_pin "archive_url")
archive_sha256=$(read_pin "archive_sha256")
if [ -z "$archive_url" ] || [ -z "$archive_sha256" ]; then
  echo "install-pinned-mage.sh: no committed Mage pin for platform ${platform} in ${toolchain_file}" >&2
  exit 1
fi

staging="$(mktemp -d)"
# Only the downloaded archive is cleaned up. The extracted directory must
# survive this script's exit: its "mage" binary is what $GITHUB_PATH now
# points at, and later workflow steps (mage Bootstrap, etc.) need it to
# still exist on disk.
archive_path="${staging}/mage-archive"
trap 'rm -f "$archive_path"' EXIT
curl -sSL --fail -o "$archive_path" "$archive_url"

if command -v sha256sum >/dev/null 2>&1; then
  actual=$(sha256sum "$archive_path" | awk '{print $1}')
else
  actual=$(shasum -a 256 "$archive_path" | awk '{print $1}')
fi
if [ "$actual" != "$archive_sha256" ]; then
  echo "install-pinned-mage.sh: checksum mismatch for ${archive_url}: got ${actual}, want ${archive_sha256}" >&2
  exit 1
fi

extract_dir="${staging}/extracted"
mkdir -p "$extract_dir"
tar -xzf "$archive_path" -C "$extract_dir"

mage_binary=$(find "$extract_dir" -type f -name 'mage' -print -quit)
if [ -z "$mage_binary" ]; then
  echo "install-pinned-mage.sh: extracted archive does not contain a mage binary" >&2
  exit 1
fi
chmod +x "$mage_binary"
dirname "$mage_binary" >> "$GITHUB_PATH"
echo "install-pinned-mage.sh: installed checksum-verified Mage ${platform} from ${archive_url}"
