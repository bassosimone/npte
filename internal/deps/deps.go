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
var All = []Dependency{
	{"ip", "iproute2"},
	{"tc", "iproute2"},
	{"iptables", "iptables"},
	{"sysctl", "procps"},
	{"modprobe", "kmod"},
	{"install", "coreutils"},
	{"systemd-run", "systemd"},
	{"systemd-nspawn", "systemd-container"},
	{"debootstrap", "debootstrap"},
}

// allowed is the allowlist derived from [All].
var allowed = func() map[string]struct{} {
	m := make(map[string]struct{}, len(All))
	for _, d := range All {
		m[d.Binary] = struct{}{}
	}
	return m
}()

// LookPath resolves name to an absolute path using the testable environment,
// but only if name is in the allowlist derived from [All]. It returns an
// error otherwise.
func LookPath(name string) (string, error) {
	if _, ok := allowed[name]; !ok {
		return "", fmt.Errorf("deps: command %q is not in the allowlist", name)
	}
	return testable.Env.LookPath(name)
}
