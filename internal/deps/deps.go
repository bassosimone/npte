// SPDX-License-Identifier: GPL-3.0-or-later

// Package deps declares the external commands that npte is allowed
// to execute and resolves their absolute path at runtime.
//
// Centralising the list serves two purposes:
//
//  1. `npte doctor` and the code that actually exec's commands share a
//     single source of truth, so the two cannot drift apart.
//  2. [LookPath] refuses to resolve names that are not in the list, which
//     restricts [subprocess] to a known-good set of binaries.
package deps

import (
	"fmt"

	"github.com/bassosimone/npte/internal/testable"
)

// Dependency describes an external command and its Debian package.
type Dependency struct {
	Binary string
	Pkg    string
}

// All is the list of external commands npte is allowed to execute.
//
// SECURITY INVARIANT: sudo MUST NEVER be added to this list — not to
// make `npte doctor` check it, not for any other reason. See the
// [SudoPath] documentation for why. A regression test enforces this.
var All = []Dependency{
	{"ip", "iproute2"},
	{"tc", "iproute2"},
	{"iptables", "iptables"},
	{"iptables-save", "iptables"},
	{"iptables-restore", "iptables"},
	{"grep", "grep"},
	{"sysctl", "procps"},
	{"modprobe", "kmod"},
	{"install", "coreutils"},
	{"env", "coreutils"},
	{"rm", "coreutils"},
	{"runuser", "util-linux"},
	{"systemd-nspawn", "systemd-container"},
	{"debootstrap", "debootstrap"},
	{"bwrap", "bubblewrap"},
}

// allowed is the allowlist derived from [All].
var allowed = func() map[string]struct{} {
	m := make(map[string]struct{}, len(All))
	for _, d := range All {
		m[d.Binary] = struct{}{}
	}
	return m
}()

// SudoPath is the fixed absolute path of sudo(8). It exists so that the
// one consumer (`npte mcp serve`, see internal/cli/mcp/session.go) and
// the `npte doctor` check cannot drift apart.
//
// SECURITY INVARIANT: sudo is deliberately NOT in [All] and MUST NEVER
// be added there. Membership in [All] means two things, and both are
// wrong for sudo:
//
//  1. [LookPath] resolves through PATH. The one consumer of sudo —
//     `npte mcp serve` — runs unprivileged, adjacent to a coding agent
//     whose writes may reach PATH directories. A planted fake "sudo"
//     earlier in PATH could not elevate, but it would silently replace
//     the entire trust bridge: commands the agent believes run under
//     the audited npte-side validators (registry, --sandbox, argv
//     contracts) would instead run under the impostor, with no signal
//     to anyone. The fixed path is the defense; resolving sudo through
//     PATH would reopen the hole.
//
//  2. Membership in [All] grants every present and future
//     subprocess.MustRun call site the right to exec the binary. sudo
//     is the privilege boundary itself: allowlisting it would let a
//     future refactor casually route composed argv across that
//     boundary through the "known-good" path, and nothing would
//     object. The whole security architecture — NOPASSWD sudoers
//     bounded by validators and the registry — assumes npte's
//     subprocess layer cannot invoke sudo.
//
// Consumers MUST exec this fixed path directly, never a PATH-resolved
// one. `npte doctor` checks this path with a stat, not with [LookPath].
const SudoPath = "/usr/bin/sudo"

// LookPath resolves name to an absolute path using the testable environment,
// but only if name is in the allowlist derived from [All]. It returns an
// error otherwise.
func LookPath(name string) (string, error) {
	if _, ok := allowed[name]; !ok {
		return "", fmt.Errorf("deps: command %q is not in the allowlist", name)
	}
	return testable.Env.LookPath(name)
}
