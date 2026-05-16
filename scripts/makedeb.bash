#!/bin/bash
set -euo pipefail

cd "$(dirname "$(dirname "$(readlink -f "$0")")")"
ver="$(git describe --tags | sed 's/^v//')~$(date -u +%Y%m%d%H%M%S)-1"
stage="$(mktemp -d)"
chmod 755 "$stage"

# TODO(bassosimone): correct only for amd64 and arm64
arch="$(go env GOARCH)"

set -x
install -d "$stage/usr/sbin"
ldflags_buildcfg="github.com/bassosimone/npte/internal/buildcfg"
go build -ldflags="-s -w -X $ldflags_buildcfg.Version=$ver -X $ldflags_buildcfg.InstallPath=/usr/sbin/npte" -o "$stage/usr/sbin/npte" .
chmod 755 "$stage/usr/sbin/npte"

install -d "$stage/usr/share/man/man8"
sed -e "s/@VERSION@/$ver/g" -e "s/@DATE@/$(date -u +%Y-%m-%d)/g" \
    man/npte.8 > "$stage/usr/share/man/man8/npte.8"
gzip -9n "$stage/usr/share/man/man8/npte.8"
chmod 644 "$stage/usr/share/man/man8/npte.8.gz"

install -d "$stage/usr/share/doc/npte"
cat >"$stage/usr/share/doc/npte/copyright" <<'EOF'
Format: https://www.debian.org/doc/packaging-manuals/copyright-format/1.0/
Upstream-Name: npte
Upstream-Contact: Simone Basso <bassosimone@gmail.com>
Source: https://github.com/bassosimone/npte

Files: *
Copyright: 2025-2026 Simone Basso <bassosimone@gmail.com>
License: GPL-3.0-or-later

License: GPL-3.0-or-later
 On Debian systems, the complete text of the GNU General Public
 License version 3 can be found in `/usr/share/common-licenses/GPL-3'.
EOF
chmod 644 "$stage/usr/share/doc/npte/copyright"

install -d "$stage/DEBIAN"
cat >"$stage/DEBIAN/control" <<EOF
Package: npte
Version: $ver
Section: net
Priority: optional
Architecture: $arch
Maintainer: Simone Basso <bassosimone@gmail.com>
Homepage: https://github.com/bassosimone/npte
Depends: bubblewrap, coreutils, debootstrap, grep, iproute2,
 iptables, kmod, procps, systemd-container, util-linux
Description: Network Performance Testing Environment
 Builds isolated Linux network namespaces wired with veth pairs and
 shaped via tc/netem, to test how a network client behaves on a
 realistic access link without real hardware.
EOF

# Generate md5sums of every shipped file (everything outside DEBIAN/).
# Paths are filesystem-relative without a leading slash, per dpkg format.
( cd "$stage" && find . -type f -not -path './DEBIAN/*' -printf '%P\n' \
  | xargs -r md5sum > DEBIAN/md5sums )
chmod 644 "$stage/DEBIAN/md5sums"

dpkg-deb --root-owner-group --build "$stage" "npte_${ver}_${arch}.deb"
