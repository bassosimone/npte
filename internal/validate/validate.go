// SPDX-License-Identifier: GPL-3.0-or-later

// Package validate contains validators for user-supplied identifiers.
package validate

import (
	"fmt"
	"net/netip"
	"regexp"
	"unicode"
)

// netnsNameMaxLen caps network-namespace names at 12 characters.
//
// Rationale: veth interfaces inside a namespace are named "if-<peer>"
// (see `npte netns connect`) and must fit in IFNAMSIZ (15 bytes
// including the trailing NUL, so 15 usable bytes). The "if-" prefix
// consumes 3 bytes, leaving 12 for the peer namespace name.
const netnsNameMaxLen = 12

var netnsNameRe = regexp.MustCompile(`^[a-z][a-z0-9]*$`)

// NetnsName reports whether name is a valid network-namespace identifier.
func NetnsName(name string) error {
	if len(name) <= 0 {
		return fmt.Errorf("netns name is empty")
	}
	if len(name) > netnsNameMaxLen {
		return fmt.Errorf("netns name %q exceeds %d characters", name, netnsNameMaxLen)
	}
	if !netnsNameRe.MatchString(name) {
		return fmt.Errorf("netns name %q must match %s", name, netnsNameRe)
	}
	return nil
}

// ifnamsiz is the kernel limit on interface-name length: 15 usable bytes
// plus the trailing NUL, per IFNAMSIZ in <linux/if.h>.
const ifnamsiz = 15

// IfaceName reports whether name is a valid network-interface identifier,
// mirroring the kernel's dev_valid_name() check in net/core/dev.c.
func IfaceName(name string) error {
	if len(name) <= 0 {
		return fmt.Errorf("iface name is empty")
	}
	if len(name) > ifnamsiz {
		return fmt.Errorf("iface name %q exceeds %d bytes", name, ifnamsiz)
	}
	if name == "." || name == ".." {
		return fmt.Errorf("iface name %q is reserved", name)
	}
	for _, r := range name {
		if r == '/' || r == ':' || unicode.IsSpace(r) {
			return fmt.Errorf("iface name %q contains forbidden character %q", name, r)
		}
	}
	return nil
}

// IPAddr reports whether s is a valid IPv4 or IPv6 address (without a prefix).
func IPAddr(s string) error {
	addr, err := netip.ParseAddr(s)
	if err != nil {
		return fmt.Errorf("ip address %q: %w", s, err)
	}
	if !addr.IsValid() {
		return fmt.Errorf("ip address %q is not valid", s)
	}
	return nil
}

// usernameMaxLen mirrors the conservative limit used by adduser(8) and
// useradd(8) on Debian-family systems: 32 bytes including the trailing NUL.
const usernameMaxLen = 32

var usernameRe = regexp.MustCompile(`^[a-z_][a-z0-9_-]*$`)

// Username reports whether s is a plausible Unix login name.
//
// The check is deliberately syntactic: we refuse obviously unsafe inputs
// (spaces, shell metacharacters, path separators, leading digits) before
// they reach `runuser -u <s>`. Whether the user actually exists is not
// checked here; `runuser` will fail loudly if it does not.
func Username(s string) error {
	if len(s) <= 0 {
		return fmt.Errorf("username is empty")
	}
	if len(s) > usernameMaxLen {
		return fmt.Errorf("username %q exceeds %d characters", s, usernameMaxLen)
	}
	if !usernameRe.MatchString(s) {
		return fmt.Errorf("username %q must match %s", s, usernameRe)
	}
	return nil
}

var envVarNameRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// EnvVarName reports whether s is a valid POSIX environment-variable name.
//
// Constraining the name shape (not the value) defends against `env` argv
// confusion: any KEY matching this regex cannot start with `-`, so it
// cannot be parsed by env(1) as an option in `env KEY=VALUE ... cmd`.
// The value is left unvalidated — env(1) copies it verbatim into envp
// without shell involvement, so arbitrary bytes are safe.
func EnvVarName(s string) error {
	if len(s) <= 0 {
		return fmt.Errorf("env var name is empty")
	}
	if !envVarNameRe.MatchString(s) {
		return fmt.Errorf("env var name %q must match %s", s, envVarNameRe)
	}
	return nil
}

var debootstrapSuiteRe = regexp.MustCompile(`^[a-z][a-z0-9]*$`)

// DebootstrapSuite reports whether s is a plausible debootstrap suite name
// (e.g. "noble", "bookworm", "trixie"). The check is syntactic: it rejects
// obviously unsafe inputs before they reach `debootstrap <suite> <target>`
// but does not verify that a corresponding script exists under
// /usr/share/debootstrap/scripts — debootstrap itself fails loud when it
// does not.
func DebootstrapSuite(s string) error {
	if len(s) <= 0 {
		return fmt.Errorf("suite name is empty")
	}
	if !debootstrapSuiteRe.MatchString(s) {
		return fmt.Errorf("suite name %q must match %s", s, debootstrapSuiteRe)
	}
	return nil
}

// CIDR reports whether s is a valid IPv4 or IPv6 prefix in "addr/len" form.
// A prefix length is required (bare addresses are rejected), and host bits
// may be set (e.g. "10.0.1.1/24" is accepted) since `ip addr add` accepts
// the same form.
func CIDR(s string) error {
	prefix, err := netip.ParsePrefix(s)
	if err != nil {
		return fmt.Errorf("cidr %q: %w", s, err)
	}
	if !prefix.IsValid() {
		return fmt.Errorf("cidr %q is not valid", s)
	}
	return nil
}
