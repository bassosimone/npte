// SPDX-License-Identifier: GPL-3.0-or-later

package validate

import (
	"fmt"
	"regexp"
	"strings"
)

// This file groups the netem flag-value validators because their
// grammars (from `man tc-netem`) are an order of magnitude more
// involved than the rest of `validate.go`. Each validator anchors a
// single regex against the trimmed-and-collapsed flag value, so
// position and arity ride in the regex itself rather than being
// re-derived at the call site.
//
// The validators return only `error`; if a caller needs the tokens
// to splice into argv, it splits with `strings.Fields` after the
// validator passes. The split is mechanical (no atom contains a
// space) so it does not need to be co-located with validation.

// TODO(bassosimone): these validators are looser than tc in a few
// corners: atomPct accepts percentages above 100, and netemDelayRe
// accepts `distribution` without a jitter, both of which tc rejects
// at runtime. This never weakens the NOPASSWD bound (the byte shapes
// stay bounded either way); it only means --dry-run can bless a
// script that fails loud when pasted. Consider tightening (move the
// distribution group inside the jitter group; range-check percent
// atoms) if that round-trip corner turns out to matter in practice.

// Atom regex fragments shared by the per-flag netem validators below.
// Each fragment is unanchored and uses non-capturing groups so it
// can be composed into a parent `^(?: ... )$` without altering group
// numbering or matching semantics. The fragments mirror the scalar
// grammars documented in `man tc-netem` (TIME, RATE, etc.) and the
// standard distribution names accepted by the kernel.
const (
	atomTime    = `\d+(?:\.\d+)?(?:us|ms|s)`
	atomPct     = `\d+(?:\.\d+)?%`
	atomNum     = `\d+`
	atomSNum    = `-?\d+`
	atomRate    = `\d+(?:\.\d+)?(?:bit|kbit|mbit|gbit|tbit)`
	atomProbPct = `\d+(?:\.\d+)?%?` // bare probability (0.1) or PCT (10%)
	atomDist    = `uniform|normal|pareto|paretonormal`
)

// netemDelayRe matches the full --delay grammar from `man tc-netem`:
//
//	TIME [ JITTER [ CORRELATION ] ] [ distribution { uniform | normal | pareto | paretonormal } ]
//
// Position and arity are encoded in the regex itself, so smuggled
// tokens (other netem knob names, shell metacharacters, unknown
// distribution names) do not match any branch.
var netemDelayRe = regexp.MustCompile(
	`^(?:` + atomTime + `)(?:\s+(?:` + atomTime + `)(?:\s+(?:` + atomPct + `))?)?` +
		`(?:\s+distribution\s+(?:` + atomDist + `))?$`)

// NetemDelay reports whether s is a valid value for the netem
// `--delay` flag. Leading/trailing whitespace is tolerated, and any
// run of whitespace between atoms counts as a separator. The caller
// splits with `strings.Fields` after this returns nil.
func NetemDelay(s string) error {
	if !netemDelayRe.MatchString(strings.TrimSpace(s)) {
		return fmt.Errorf("invalid --delay value %q", s)
	}
	return nil
}

// netemLossRe matches the full --loss grammar from `man tc-netem`,
// across the three model branches:
//
//	random:  [random] PERCENT [ CORRELATION ]
//	state:   state P13 [ P31 [ P32 [ P23 [ P14 ] ] ] ]
//	gemodel: gemodel PERCENT [ R [ 1-H [ 1-K ] ] ]
//
// The state model takes percentages; gemodel takes either bare
// probabilities (e.g. 0.1) or percentages — atomProbPct accepts both.
var netemLossRe = regexp.MustCompile(
	`^(?:` +
		// random branch (with the literal `random` optional)
		`(?:random\s+)?(?:` + atomPct + `)(?:\s+(?:` + atomPct + `))?` +
		`|` +
		// state branch: state PCT [PCT [PCT [PCT [PCT]]]]
		`state(?:\s+(?:` + atomPct + `)){1,5}` +
		`|` +
		// gemodel branch: gemodel PROBPCT [PROBPCT [PROBPCT [PROBPCT]]]
		`gemodel(?:\s+(?:` + atomProbPct + `)){1,4}` +
		`)$`)

// NetemLoss reports whether s is a valid value for the netem
// `--loss` flag. Leading/trailing whitespace is tolerated, and any
// run of whitespace between atoms counts as a separator. The caller
// splits with `strings.Fields` after this returns nil.
func NetemLoss(s string) error {
	if !netemLossRe.MatchString(strings.TrimSpace(s)) {
		return fmt.Errorf("invalid --loss value %q", s)
	}
	return nil
}

// netemLimitRe matches the full --limit grammar: a single
// non-negative integer (the packet count cap, see `man tc-netem`).
var netemLimitRe = regexp.MustCompile(`^(?:` + atomNum + `)$`)

// NetemLimit reports whether s is a valid value for the netem
// `--limit` flag. Leading/trailing whitespace is tolerated.
func NetemLimit(s string) error {
	if !netemLimitRe.MatchString(strings.TrimSpace(s)) {
		return fmt.Errorf("invalid --limit value %q", s)
	}
	return nil
}

// netemRateRe matches the full --rate grammar from `man tc-netem`:
//
//	RATE [ PACKETOVERHEAD [ CELLSIZE [ CELLOVERHEAD ] ] ]
//
// PACKETOVERHEAD and CELLOVERHEAD may be negative; CELLSIZE is a
// non-negative byte count.
var netemRateRe = regexp.MustCompile(
	`^(?:` + atomRate + `)` +
		`(?:\s+(?:` + atomSNum + `)` +
		`(?:\s+(?:` + atomNum + `)` +
		`(?:\s+(?:` + atomSNum + `))?)?)?$`)

// NetemRate reports whether s is a valid value for the netem
// `--rate` flag. Leading/trailing whitespace is tolerated, and any
// run of whitespace between atoms counts as a separator.
func NetemRate(s string) error {
	if !netemRateRe.MatchString(strings.TrimSpace(s)) {
		return fmt.Errorf("invalid --rate value %q", s)
	}
	return nil
}

// netemSlotRe matches the full --slot grammar from `man tc-netem`:
//
//	{ MIN_DELAY [ MAX_DELAY ] | distribution { uniform | normal | pareto | paretonormal } DELAY JITTER }
//	  [ packets PACKETS ] [ bytes BYTES ]
//
// The kernel also accepts `distribution FILE` (path to a tabulated
// distribution under /usr/lib/tc), but we deliberately do not. A
// FILE value smuggled into a NOPASSWD invocation would let the
// caller pick which distribution table tc reads, and the path
// resolution happens inside tc — the safer floor is to allow only
// the four named distributions baked into the kernel, which is what
// the bracketed list above already constrains.
var netemSlotRe = regexp.MustCompile(
	`^(?:` +
		// min/max-delay form
		`(?:` + atomTime + `)(?:\s+(?:` + atomTime + `))?` +
		`|` +
		// distribution form
		`distribution\s+(?:` + atomDist + `)\s+(?:` + atomTime + `)\s+(?:` + atomTime + `)` +
		`)` +
		// optional `packets NUM` and/or `bytes NUM` suffixes
		`(?:\s+packets\s+(?:` + atomNum + `))?` +
		`(?:\s+bytes\s+(?:` + atomNum + `))?$`)

// NetemSlot reports whether s is a valid value for the netem
// `--slot` flag. Leading/trailing whitespace is tolerated, and any
// run of whitespace between atoms counts as a separator.
func NetemSlot(s string) error {
	if !netemSlotRe.MatchString(strings.TrimSpace(s)) {
		return fmt.Errorf("invalid --slot value %q", s)
	}
	return nil
}
