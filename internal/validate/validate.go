// SPDX-License-Identifier: GPL-3.0-or-later

// Package validate contains validators for user-supplied identifiers.
package validate

import (
	"fmt"
	"regexp"
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
