// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"fmt"
	"net"
	"path/filepath"
	"regexp"
)

// baseDir is the system-level directory for npte projects.
var baseDir = filepath.Join("/", "var", "local", "npte")

// statePath returns the path to the network state file for a project.
func statePath(proj string) string {
	return filepath.Join(baseDir, proj, "state", "net.json")
}

// lockPath returns the path to the state lock file for a project.
func lockPath(proj string) string {
	return filepath.Join(baseDir, proj, "state", "net.lock")
}

// treePath returns the path to a container filesystem tree.
func treePath(proj, name string) string {
	return filepath.Join(baseDir, proj, "trees", name)
}

// netState is the full network state stored in the state file.
type netState struct {
	// Project is the project name (matches the directory name under baseDir).
	// It is also used as a prefix for kernel resource names (namespaces, interfaces).
	Project string `json:"project"`

	// RouterSubnet is the CIDR for the router's host-facing veth.
	RouterSubnet string `json:"router_subnet"`

	// NextSubnetIndex is the next index to use when auto-allocating
	// a subnet for a new endpoint (e.g., 1 → 10.0.1.0/24).
	NextSubnetIndex int `json:"next_subnet_index"`

	// Hosts maps host name to host state.
	Hosts map[string]*hostState `json:"hosts"`
}

// hostState is the per-host state within the network.
type hostState struct {
	Name   string `json:"name"`
	Subnet string `json:"subnet"`
}

// mustLockNetState acquires the state lock file. The returned function
// must be called to release the lock.
func mustLockNetState(proj string) func() {
	lp := lockPath(proj)
	logDetails("npte: acquire state lock %s\n", lp)
	env.LogFatalOnError0(env.MkdirAll(filepath.Dir(lp), 0755))
	unlock, err := env.LockFile(lp)
	if err != nil {
		logAlways("npte: cannot acquire state lock: %s\n", err)
		env.Exit(1)
	}
	return func() {
		logDetails("npte: release state lock %s\n", lp)
		unlock()
	}
}

var identRe = regexp.MustCompile(`^[a-z][a-z0-9]*$`)

// validateIdent checks that s is a non-empty string of lowercase letters.
func validateIdent(s string) error {
	if !identRe.MatchString(s) {
		return fmt.Errorf("invalid identifier %q: must start with a lowercase letter and contain only lowercase letters and digits", s)
	}
	return nil
}

// maxIfNameLen is the Linux IFNAMSIZ limit (including the NUL terminator).
const maxIfNameLen = 15

// validateEndpointName checks that name is a valid identifier and that
// the resulting interface names (e.g., proj-name-s) fit in IFNAMSIZ.
func validateEndpointName(proj, name string) error {
	if err := validateIdent(name); err != nil {
		return err
	}
	ifname := proj + "-" + name + "-s"
	if len(ifname) > maxIfNameLen {
		return fmt.Errorf("interface name %q exceeds %d chars", ifname, maxIfNameLen)
	}
	return nil
}

// validateProject checks that proj is a valid identifier and short enough
// for the longest interface name (proj + "-router-h", 9 chars suffix).
func validateProject(proj string) error {
	if err := validateIdent(proj); err != nil {
		return err
	}
	if len(proj)+9 > maxIfNameLen {
		return fmt.Errorf("project name %q is too long (max %d chars)", proj, maxIfNameLen-9)
	}
	return nil
}

// nsName returns the kernel namespace name for a host.
func nsName(proj, name string) string {
	return proj + "-" + name
}

// nsPath returns the filesystem path for a network namespace.
func nsPath(proj, name string) string {
	return filepath.Join("/", "var", "run", "netns", nsName(proj, name))
}

// saveNetState writes the network state to disk.
func saveNetState(proj string, ns *netState) error {
	sp := statePath(proj)
	if err := env.MkdirAll(filepath.Dir(sp), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(ns, "", "  ")
	if err != nil {
		return err
	}
	return env.WriteFile(sp, append(data, '\n'), 0644)
}

// loadNetState reads the network state from disk.
func loadNetState(proj string) (*netState, error) {
	data, err := env.ReadFile(statePath(proj))
	if err != nil {
		return nil, err
	}
	ns := &netState{}
	if err := json.Unmarshal(data, ns); err != nil {
		return nil, err
	}
	return ns, nil
}

// mustLoadNetState loads the network state or exits with an error.
func mustLoadNetState(proj string) *netState {
	ns, err := loadNetState(proj)
	if err != nil {
		logAlways("npte: cannot load network state: %s\n", err)
		logAlways("npte: run `npte net init <project>' first\n")
		env.Exit(1)
	}
	return ns
}

// ipWithOffset returns the IP at a given offset within a subnet.
//
// For example, ipWithOffset(10.0.1.0/24, 1) returns 10.0.1.1.
func ipWithOffset(ipNet *net.IPNet, offset int) net.IP {
	ip := make(net.IP, len(ipNet.IP))
	copy(ip, ipNet.IP)

	// Work on the 4-byte form for IPv4
	ip4 := ip.To4()
	if ip4 != nil {
		ip4[3] += byte(offset)
		return ip4
	}

	// IPv6: add offset to the last byte
	ip[15] += byte(offset)
	return ip
}
