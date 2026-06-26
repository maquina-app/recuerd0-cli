#!/bin/sh
# Ad-hoc codesign darwin (Mach-O) binaries during the goreleaser build, so the
# released macOS binaries actually run on Apple Silicon: arm64 refuses to exec an
# unsigned Mach-O and SIGKILLs it. brew/curl installs strip or never set the
# quarantine xattr, so an ad-hoc signature is sufficient for the CLI to run — a
# real Developer ID signature + notarization (for Gatekeeper on quarantined
# downloads) is a follow-up that needs Apple credentials.
#
# Invoked from .goreleaser.yaml's build post-hook for every target with the built
# binary path and its GOOS; a no-op for non-darwin targets. Requires `codesign`,
# i.e. releasing from a Mac (see README "Release").
set -eu

path="$1"
os="${2:-}"

# Only darwin binaries get signed. Fall back to a path check if GOOS wasn't passed.
case "$os" in
	darwin) ;;
	"") case "$path" in *darwin*) ;; *) exit 0 ;; esac ;;
	*) exit 0 ;;
esac

codesign --force --sign - "$path"
echo "ad-hoc signed $path"
