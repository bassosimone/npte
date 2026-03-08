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

// defaultPrefix is the default /16 prefix for new projects.
const defaultPrefix = "10.0.0.0/16"

// projectDir returns the root directory for a project.
func projectDir(proj string) string {
	return filepath.Join(baseDir, proj)
}

// configPath returns the path to the network config file for a project.
func configPath(proj string) string {
	return filepath.Join(baseDir, proj, "config", "netns.json")
}

// lockPath returns the path to the config lock file for a project.
func lockPath(proj string) string {
	return filepath.Join(baseDir, proj, "config", "netns.lock")
}

// resolvConfPath returns the path to the project's resolv.conf.
func resolvConfPath(proj string) string {
	return filepath.Join(baseDir, proj, "config", "resolv.conf")
}

// treePath returns the path to a container filesystem tree.
func treePath(proj, name string) string {
	return filepath.Join(baseDir, proj, "trees", name)
}

// netnsConfig is the network configuration stored in the config file.
type netnsConfig struct {
	// Prefix is the /16 address block for this project (e.g., "10.0.0.0/16").
	// Within this block, /24 subnets are allocated: index 0 is the router-to-host
	// link, and indices 1+ are endpoint subnets.
	Prefix string `json:"prefix"`

	// NextSubnetIndex is the next index to use when auto-allocating
	// a /24 subnet for a new endpoint.
	NextSubnetIndex int `json:"next_subnet_index"`

	// Hosts maps host name to host config.
	Hosts map[string]*hostConfig `json:"hosts"`
}

// validatePrefix checks that s is a valid /16 CIDR prefix.
func validatePrefix(s string) (*net.IPNet, error) {
	_, prefix, err := net.ParseCIDR(s)
	if err != nil {
		return nil, fmt.Errorf("invalid prefix %q: %s", s, err)
	}
	ones, _ := prefix.Mask.Size()
	if ones != 16 {
		return nil, fmt.Errorf("prefix must be a /16, got /%d", ones)
	}
	return prefix, nil
}

// mustSubnet returns the /24 subnet at the given index within the project's prefix.
// Index 0 is the router-to-host link; indices 1+ are endpoint subnets.
// Exits the process if the prefix is invalid.
func (cfg *netnsConfig) mustSubnet(index int) *net.IPNet {
	prefix, err := validatePrefix(cfg.Prefix)
	if err != nil {
		logError("npte: %s", err)
		env.Exit(1)
	}
	if index < 0 || index > 255 {
		logError("npte: subnet index %d out of range (0-255)", index)
		env.Exit(1)
	}
	ip := make(net.IP, 4)
	copy(ip, prefix.IP.To4())
	ip[2] = byte(index)
	return &net.IPNet{
		IP:   ip,
		Mask: net.CIDRMask(24, 32),
	}
}

// hostConfig is the per-host configuration within the network.
type hostConfig struct {
	Name        string `json:"name"`
	SubnetIndex int    `json:"subnet_index"`
}

// mustLockNetnsConfig acquires the config lock file. The returned function
// must be called to release the lock.
func mustLockNetnsConfig(proj string) func() {
	lp := lockPath(proj)
	logDetails("npte: acquire config lock %s", lp)
	env.LogFatalOnError0(env.MkdirAll(filepath.Dir(lp), 0755))
	unlock, err := env.LockFile(lp)
	if err != nil {
		logError("npte: cannot acquire config lock: %s", err)
		env.Exit(1)
	}
	return func() {
		logDetails("npte: release config lock %s", lp)
		unlock()
	}
}

var identRe = regexp.MustCompile(`^[a-z][a-z0-9]*$`)
var userRe = regexp.MustCompile(`^[a-z_][a-z0-9_-]*$`)

// validateIdent checks that s is a valid identifier.
func validateIdent(s string) error {
	if !identRe.MatchString(s) {
		return fmt.Errorf("invalid identifier %q: must start with a lowercase letter and contain only lowercase letters and digits", s)
	}
	return nil
}

// maxIfNameLen is the Linux IFNAMSIZ limit (including the NUL terminator).
const maxIfNameLen = 15

// validateUser checks that s is a valid Unix username.
func validateUser(s string) error {
	if s == "" {
		return fmt.Errorf("user name must not be empty")
	}
	if len(s) > 32 {
		return fmt.Errorf("user name %q is too long (max 32 chars)", s)
	}
	if !userRe.MatchString(s) {
		return fmt.Errorf("invalid user name %q: must start with a lowercase letter or underscore "+
			"and contain only lowercase letters, digits, underscores, and hyphens", s)
	}
	return nil
}

var suiteRe = regexp.MustCompile(`^[a-z][a-z0-9]*$`)

// validateSuite checks that s is a valid debootstrap suite name.
func validateSuite(s string) error {
	if !suiteRe.MatchString(s) {
		return fmt.Errorf("invalid suite name %q: must start with a lowercase letter "+
			"and contain only lowercase letters and digits", s)
	}
	return nil
}

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

// saveNetnsConfig writes the network config to disk.
func saveNetnsConfig(proj string, cfg *netnsConfig) error {
	cp := configPath(proj)
	if err := env.MkdirAll(filepath.Dir(cp), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return env.WriteFile(cp, append(data, '\n'), 0644)
}

// validateNetnsConfig checks that the loaded config has valid fields.
func validateNetnsConfig(proj string, cfg *netnsConfig) error {
	if _, err := validatePrefix(cfg.Prefix); err != nil {
		return fmt.Errorf("config: %w", err)
	}
	if cfg.NextSubnetIndex < 1 || cfg.NextSubnetIndex > 256 {
		return fmt.Errorf("config: next_subnet_index %d out of range (1-256)", cfg.NextSubnetIndex)
	}
	for name, hs := range cfg.Hosts {
		if err := validateEndpointName(proj, name); err != nil {
			return fmt.Errorf("config: host %q: %w", name, err)
		}
		if hs.SubnetIndex < 1 || hs.SubnetIndex > 255 {
			return fmt.Errorf("config: host %q: subnet_index %d out of range (1-255)", name, hs.SubnetIndex)
		}
	}
	return nil
}

// loadNetnsConfig reads the network config from disk.
func loadNetnsConfig(proj string) (*netnsConfig, error) {
	data, err := env.ReadFile(configPath(proj))
	if err != nil {
		return nil, err
	}
	cfg := &netnsConfig{}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	if err := validateNetnsConfig(proj, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// mustLoadNetnsConfig loads the network config or exits with an error.
func mustLoadNetnsConfig(proj string) *netnsConfig {
	cfg, err := loadNetnsConfig(proj)
	if err != nil {
		logError("npte: cannot load netns config: %s", err)
		logError("npte: have you run %q and %q?", "npte project create", "npte netns create")
		env.Exit(1)
	}
	return cfg
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
