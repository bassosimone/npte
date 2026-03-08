// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
	"time"
)

// halfRTT parses an RTT duration string and returns the one-way delay
// formatted for tc (e.g., "60ms" → "30ms", "5ms" → "2500us").
func halfRTT(rtt string) (string, error) {
	d, err := time.ParseDuration(rtt)
	if err != nil {
		return "", fmt.Errorf("invalid RTT %q: %w", rtt, err)
	}
	if d <= 0 {
		return "", fmt.Errorf("RTT must be positive, got %q", rtt)
	}
	half := d / 2
	ms := half.Milliseconds()
	if time.Duration(ms)*time.Millisecond == half {
		return fmt.Sprintf("%dms", ms), nil
	}
	return fmt.Sprintf("%dus", half.Microseconds()), nil
}
