#!/bin/bash
# Build deps: git, go, objdump (computes the libc6 dependency), uv
# (stages the Python package), dpkg-deb, lintian.
# These are build-machine tools only: do not add them to internal/deps.
set -euo pipefail

# Debian policy wants 0755 directories; `install -d` applies the build
# user's umask to the intermediate directories it creates.
umask 022

cd "$(dirname "$(dirname "$(readlink -f "$0")")")"
ver="$(git describe --tags | sed 's/^v//')~$(date -u +%Y%m%d%H%M%S)-1"
stage="$(mktemp -d)"
chmod 755 "$stage"

# TODO(bassosimone): correct only for amd64 and arm64
arch="$(go env GOARCH)"

set -x

# Build and install the binary.
#
# -buildmode=pie yields a PIE so the kernel can randomize the load
# address (ASLR); npte runs as root, so opt into hardening.
install -d "$stage/usr/sbin"
ldflags_buildcfg="github.com/bassosimone/npte/internal/buildcfg"
go build -buildmode=pie -ldflags="-s -w -X $ldflags_buildcfg.Version=$ver -X $ldflags_buildcfg.InstallPath=/usr/sbin/npte" -o "$stage/usr/sbin/npte" .
chmod 755 "$stage/usr/sbin/npte"

# Compute the libc6 version the binary actually requires: the highest
# GLIBC_x.y symbol version it references. This mirrors what
# dpkg-shlibdeps derives for real Debian packages.
libc_ver="$(objdump -T "$stage/usr/sbin/npte" \
    | grep -oE 'GLIBC_[0-9.]+' | sed 's/^GLIBC_//' | sort -uV | tail -1)"

# Build and install Python package.
install -d "$stage/usr/lib/python3/dist-packages"
uv pip install --link-mode=copy --target "$stage/x" ./python
mv "$stage/x/npte" "$stage/usr/lib/python3/dist-packages"
rm -r "$stage/x"

# Install manpage.
install -d "$stage/usr/share/man/man8"
sed -e "s/@VERSION@/$ver/g" -e "s/@DATE@/$(date -u +%Y-%m-%d)/g" \
    dist/unix/usr/share/man/man8/npte.8 > "$stage/usr/share/man/man8/npte.8"
gzip -9n "$stage/usr/share/man/man8/npte.8"
chmod 644 "$stage/usr/share/man/man8/npte.8.gz"

# Install copyright.
install -d "$stage/usr/share/doc/npte"
install -m 644 dist/debian/copyright "$stage/usr/share/doc/npte/"

# Install lintian overrides.
install -d "$stage/usr/share/lintian/overrides"
install -m 644 dist/debian/lintian-overrides "$stage/usr/share/lintian/overrides/npte"

# Install control file with substitutions.
#
# Note: binary control files do not allow comments: strip them.
install -d "$stage/DEBIAN"
sed -e "s/@VERSION@/$ver/g" -e "s/@ARCH@/$arch/g" \
    -e "s/@LIBC@/$libc_ver/g" -e '/^#/d' \
    dist/debian/control > "$stage/DEBIAN/control"

# Install maintainer scripts.
install -m 755 dist/debian/postinst "$stage/DEBIAN/"
install -m 755 dist/debian/prerm "$stage/DEBIAN/"

# Generate md5sums of every shipped file (everything outside DEBIAN/).
# Paths are filesystem-relative without a leading slash, per dpkg format.
( cd "$stage" && find . -type f -not -path './DEBIAN/*' -printf '%P\n' \
  | xargs -r md5sum > DEBIAN/md5sums )
chmod 644 "$stage/DEBIAN/md5sums"

dpkg-deb --root-owner-group --build "$stage" "npte_${ver}_${arch}.deb"

# Check the package for policy violations.
lintian --tag-display-limit 0 "npte_${ver}_${arch}.deb"
