#!/bin/bash
set -euo pipefail

cd "$(dirname "$(dirname "$(readlink -f "$0")")")"
ver="$(git describe --tags 2>/dev/null | sed 's/^v//')~$(date -u +%Y%m%d%H%M%S)-1"
stage="$(mktemp -d)"
chmod 755 "$stage"

# TODO(bassosimone): correct only for amd64 and arm64
arch="$(go env GOARCH)"

set -x

# Build the binary.
install -d "$stage/usr/sbin"
ldflags_buildcfg="github.com/bassosimone/npte/internal/buildcfg"
go build -ldflags="-s -w -X $ldflags_buildcfg.Version=$ver -X $ldflags_buildcfg.InstallPath=/usr/sbin/npte" -o "$stage/usr/sbin/npte" .
chmod 755 "$stage/usr/sbin/npte"

# Install manpage.
install -d "$stage/usr/share/man/man8"
sed -e "s/@VERSION@/$ver/g" -e "s/@DATE@/$(date -u +%Y-%m-%d)/g" \
    man/npte.8 > "$stage/usr/share/man/man8/npte.8"
gzip -9n "$stage/usr/share/man/man8/npte.8"
chmod 644 "$stage/usr/share/man/man8/npte.8.gz"

# Install copyright.
install -d "$stage/usr/share/doc/npte"
install -m 644 dist/debian/copyright "$stage/usr/share/doc/npte/"

# Install control file with substitutions.
install -d "$stage/DEBIAN"
sed -e "s/@VERSION@/$ver/g" -e "s/@ARCH@/$arch/g" \
    dist/debian/control > "$stage/DEBIAN/control"

# Generate md5sums of every shipped file (everything outside DEBIAN/).
# Paths are filesystem-relative without a leading slash, per dpkg format.
( cd "$stage" && find . -type f -not -path './DEBIAN/*' -printf '%P\n' \
  | xargs -r md5sum > DEBIAN/md5sums )
chmod 644 "$stage/DEBIAN/md5sums"

dpkg-deb --root-owner-group --build "$stage" "npte_${ver}_${arch}.deb"
