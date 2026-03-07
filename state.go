// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"fmt"
	"net"
	"path/filepath"
	"regexp"
	"strings"
)

// netState is the full network state stored in .npte/state/net.json.
type netState struct {
	// Prefix is the deployment prefix (read from .npte/config/name).
	Prefix string `json:"prefix"`

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

var netStatePath = filepath.Join(".npte", "state", "net.json")
var netStateLockPath = filepath.Join(".npte", "state", "net.lock")

// mustLockNetState acquires the state lock file. The returned function
// must be called to release the lock.
func mustLockNetState() func() {
	logDetails("npte: acquire state lock %s\n", netStateLockPath)
	unlock, err := env.LockFile(netStateLockPath)
	if err != nil {
		logAlways("npte: cannot acquire state lock: %s\n", err)
		env.Exit(1)
	}
	return func() {
		logDetails("npte: release state lock %s\n", netStateLockPath)
		unlock()
	}
}

var identRe = regexp.MustCompile(`^[a-z]+$`)

// validateIdent checks that s is a non-empty string of lowercase letters.
func validateIdent(s string) error {
	if !identRe.MatchString(s) {
		return fmt.Errorf("invalid identifier %q: must be non-empty lowercase letters only", s)
	}
	return nil
}

// maxIfNameLen is the Linux IFNAMSIZ limit (including the NUL terminator).
const maxIfNameLen = 15

// validateEndpointName checks that name is a valid identifier and that
// the resulting interface names (e.g., pfx-name-s) fit in IFNAMSIZ.
func validateEndpointName(pfx, name string) error {
	if err := validateIdent(name); err != nil {
		return err
	}
	ifname := pfx + "-" + name + "-s"
	if len(ifname) > maxIfNameLen {
		return fmt.Errorf("interface name %q exceeds %d chars", ifname, maxIfNameLen)
	}
	return nil
}

// mustLoadPrefix reads the deployment prefix from .npte/config/name.
func mustLoadPrefix() string {
	prefixPath := filepath.Join(".npte", "config", "name")
	data, err := env.ReadFile(prefixPath)
	if err != nil {
		logAlways("npte: cannot read %s: %s\n", prefixPath, err)
		logAlways("npte: run `npte init' first\n")
		env.Exit(1)
	}
	pfx := strings.TrimSpace(string(data))
	if err := validateIdent(pfx); err != nil {
		logAlways("npte: invalid prefix: %s\n", err)
		env.Exit(1)
	}
	// longest router interface name: pfx + "-router-h" (9 chars suffix)
	if len(pfx)+9 > maxIfNameLen {
		logAlways("npte: prefix %q is too long (max %d chars)\n", pfx, maxIfNameLen-9)
		env.Exit(1)
	}
	return pfx
}

// nsName returns the kernel namespace name for a host.
func nsName(pfx, name string) string {
	return pfx + "-" + name
}

// nsPath returns the filesystem path for a network namespace.
func nsPath(pfx, name string) string {
	return filepath.Join("/", "var", "run", "netns", nsName(pfx, name))
}

// saveNetState writes the network state to disk.
func saveNetState(ns *netState) error {
	if err := env.MkdirAll(filepath.Dir(netStatePath), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(ns, "", "  ")
	if err != nil {
		return err
	}
	return env.WriteFile(netStatePath, append(data, '\n'), 0644)
}

// loadNetState reads the network state from disk.
func loadNetState() (*netState, error) {
	data, err := env.ReadFile(netStatePath)
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
func mustLoadNetState() *netState {
	ns, err := loadNetState()
	if err != nil {
		logAlways("npte: cannot load network state: %s\n", err)
		logAlways("npte: run `npte net init' first\n")
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
