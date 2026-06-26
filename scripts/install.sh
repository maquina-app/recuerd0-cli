#!/bin/sh
# Recuerd0 CLI installer — POSIX sh, no bashisms, no jq.
#
#   curl -fsSL https://github.com/maquina-app/recuerd0-cli/releases/latest/download/install.sh | sh
#
# It detects your OS/arch, downloads the matching recuerd0-<os>-<arch> release
# asset, verifies it against the release checksums.txt (sha256), and installs the
# binary to /usr/local/bin (or $HOME/.local/bin if that isn't writable).
set -eu

REPO="maquina-app/recuerd0-cli"
BINARY="recuerd0"

# --- helpers ---------------------------------------------------------------

err() {
	echo "recuerd0 install: $*" >&2
	exit 1
}

info() {
	echo "recuerd0 install: $*" >&2
}

have() {
	command -v "$1" >/dev/null 2>&1
}

# download URL OUTFILE — fetch with curl or wget, whichever exists.
download() {
	url="$1"
	out="$2"
	if have curl; then
		curl -fsSL "$url" -o "$out"
	elif have wget; then
		wget -qO "$out" "$url"
	else
		err "need curl or wget to download"
	fi
}

# fetch_stdout URL — fetch to stdout (for the releases JSON).
fetch_stdout() {
	url="$1"
	if have curl; then
		curl -fsSL "$url"
	elif have wget; then
		wget -qO- "$url"
	else
		err "need curl or wget to download"
	fi
}

# --- OS / arch detection ---------------------------------------------------

detect_os() {
	os=$(uname -s)
	case "$os" in
	Darwin) echo "darwin" ;;
	Linux) echo "linux" ;;
	*) err "unsupported OS: $os (recuerd0 ships for macOS and Linux)" ;;
	esac
}

detect_arch() {
	arch=$(uname -m)
	case "$arch" in
	x86_64 | amd64) echo "amd64" ;;
	arm64 | aarch64) echo "arm64" ;;
	*) err "unsupported architecture: $arch (recuerd0 ships amd64 and arm64)" ;;
	esac
}

# --- release resolution (no jq) --------------------------------------------

# latest_tag — parse the GitHub releases/latest JSON for "tag_name" with awk.
latest_tag() {
	api="https://api.github.com/repos/${REPO}/releases/latest"
	fetch_stdout "$api" |
		awk -F'"' '/"tag_name"/ { print $4; exit }'
}

# --- checksum verification -------------------------------------------------

# sha256_of FILE — print the file's sha256, using whatever tool is present.
sha256_of() {
	file="$1"
	if have sha256sum; then
		sha256sum "$file" | awk '{print $1}'
	elif have shasum; then
		shasum -a 256 "$file" | awk '{print $1}'
	else
		err "need sha256sum or shasum to verify the download"
	fi
}

# verify_checksum FILE CHECKSUMS ASSET — confirm FILE's sha256 matches the
# entry for ASSET in the checksums.txt file. Fails loudly on mismatch.
verify_checksum() {
	file="$1"
	checksums="$2"
	asset="$3"

	expected=$(awk -v a="$asset" '$2 == a || $2 == "*"a { print $1; exit }' "$checksums")
	if [ -z "$expected" ]; then
		err "no checksum entry for $asset in checksums.txt"
	fi
	actual=$(sha256_of "$file")
	if [ "$expected" != "$actual" ]; then
		err "checksum mismatch for $asset (expected $expected, got $actual)"
	fi
	info "checksum verified ($asset)"
}

# --- install destination ---------------------------------------------------

# install_dir — /usr/local/bin if writable, else $HOME/.local/bin.
install_dir() {
	if [ -w /usr/local/bin ] 2>/dev/null; then
		echo "/usr/local/bin"
	elif [ -d /usr/local/bin ] && [ "$(id -u)" = "0" ]; then
		echo "/usr/local/bin"
	else
		echo "${HOME}/.local/bin"
	fi
}

# --- main ------------------------------------------------------------------

main() {
	os=$(detect_os)
	arch=$(detect_arch)
	info "detected ${os}/${arch}"

	tag=$(latest_tag)
	[ -n "$tag" ] || err "could not determine the latest release tag"
	info "latest release: $tag"

	asset="${BINARY}-${os}-${arch}.tar.gz"
	base="https://github.com/${REPO}/releases/download/${tag}"

	tmp=$(mktemp -d 2>/dev/null || mktemp -d -t recuerd0)
	trap 'rm -rf "$tmp"' EXIT INT TERM

	info "downloading $asset"
	download "${base}/${asset}" "${tmp}/${asset}"
	download "${base}/checksums.txt" "${tmp}/checksums.txt"

	verify_checksum "${tmp}/${asset}" "${tmp}/checksums.txt" "$asset"

	info "extracting"
	tar -xzf "${tmp}/${asset}" -C "$tmp"
	[ -f "${tmp}/${BINARY}" ] || err "archive did not contain a ${BINARY} binary"

	dir=$(install_dir)
	mkdir -p "$dir"
	if install -m 755 "${tmp}/${BINARY}" "${dir}/${BINARY}" 2>/dev/null; then
		:
	else
		# Fall back to cp+chmod if `install` is unavailable.
		cp "${tmp}/${BINARY}" "${dir}/${BINARY}"
		chmod 755 "${dir}/${BINARY}"
	fi

	info "installed ${BINARY} to ${dir}/${BINARY}"

	case ":${PATH}:" in
	*":${dir}:"*) ;;
	*) info "note: ${dir} is not on your PATH — add it to use \`recuerd0\` directly" ;;
	esac

	cat >&2 <<EOF

recuerd0 ${tag} installed.

Next steps:
  recuerd0 account add personal --token YOUR_API_TOKEN   # store your API token
  recuerd0 workspace list                                # confirm it works
EOF
}

main "$@"
