// SPDX-License-Identifier: GPL-3.0-or-later

package star

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/bassosimone/npte/internal/logx"
	"github.com/bassosimone/npte/internal/testable"
	"github.com/bassosimone/runtimex"
	"github.com/bassosimone/textwrap"
	"github.com/bassosimone/vflag"
)

// linkShape describes the shape of one direction of the star's access link.
//
// Exactly one of two combinations is valid per shape (enforced at init):
//
//   - rate set, child empty: the rate cap and optional FIFO depth live
//     inside netem (the "dumb pipe" shape). limit/loss may also be set.
//
//   - child set, rate and limit empty: netem is demoted to a pure delay
//     (and optional loss) element; the rate cap and queue discipline live
//     in the child qdisc (the "managed pipe" shape, typically cake).
//
// Mixing rate on netem and a child qdisc is the chapter-050 trap: the
// bottleneck queue lives inside netem and the child never sees a backlog.
// The init-time assertion rejects that combination.
type linkShape struct {
	delay string // required
	rate  string // netem rate; mutually exclusive with child
	limit string // netem FIFO depth in packets; only with rate
	loss  string // optional, may appear with either shape
	child string // child qdisc string; mutually exclusive with rate
}

// profile is a named pair of link shapes, one per direction of the star's
// access link. downlink shapes router→client traffic (installed on
// router/if-client); uplink shapes router→server traffic (installed on
// router/if-server).
type profile struct {
	description string
	downlink    linkShape
	uplink      linkShape
}

// profiles is the curated set of named shaping profiles.
//
// To add a profile: extend this map. The init() below validates every
// entry and panics if a profile is malformed — adding a new profile
// is a compile-then-boot check, not a runtime surprise.
var profiles = map[string]profile{
	"4g-bloated": {
		description: "Pre-AQM 4G: 50ms idle RTT, 30/10 Mbit/s, dumb netem FIFO " +
			"sized for ~1s of downlink and ~2s of uplink bloat. Produces the " +
			"textbook bufferbloat sawtooth under load.",
		downlink: linkShape{delay: "25ms", rate: "30mbit", limit: "2500"},
		uplink:   linkShape{delay: "25ms", rate: "10mbit", limit: "1700"},
	},
	"4g-managed": {
		description: "Modern 4G with AQM: same 50ms idle RTT and 30/10 Mbit/s " +
			"caps as 4g-bloated, but the rate cap lives in a cake child qdisc " +
			"that also handles per-flow fair queueing and CoDel-style early " +
			"drops. Loaded RTT stays close to idle.",
		downlink: linkShape{delay: "25ms", child: "cake bandwidth 30mbit"},
		uplink:   linkShape{delay: "25ms", child: "cake bandwidth 10mbit"},
	},
}

func init() {
	for _, p := range profiles {
		for _, shape := range []linkShape{p.downlink, p.uplink} {
			runtimex.Assert(shape.delay != "")
			hasNetemRate := shape.rate != ""
			hasChild := shape.child != ""
			runtimex.Assert(hasNetemRate != hasChild) // exactly one
			if !hasNetemRate {
				runtimex.Assert(shape.limit == "") // limit is a netem-FIFO knob
			}
		}
	}
}

// profileNames returns every defined profile name in alphabetical order,
// so help output and error messages are deterministic.
func profileNames() []string {
	names := make([]string, 0, len(profiles))
	for name := range profiles {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// netemMain is the main of the `star netem` subcommand.
func netemMain(ctx context.Context, args []string) error {
	env := testable.Env

	fset := vflag.NewFlagSet("npte star netem", vflag.ExitOnError)
	usage := vflag.NewDefaultUsagePrinter()

	// Build the "Available profiles:" block dynamically so the help text
	// cannot drift out of sync with the profiles map.
	desc := []string{
		"Shape the star's access links with a named profile. Implemented as " +
			"a composition of `npte netem clear` and `npte netem apply` calls " +
			"against router/if-client (shapes router→client, i.e. downlink) " +
			"and router/if-server (shapes router→server, i.e. uplink).",
		"With --profile <name>, both directions are first cleared (so " +
			"re-running with a different profile Just Works) and then " +
			"re-shaped per the named profile. With --profile \"\" (the " +
			"default), the interfaces are only cleared; nothing is re-applied.",
		"Available profiles:",
	}
	// Wrap each profile entry so continuation lines stay aligned in the
	// rendered help. vflag's div1 takes an entry starting with 4+ spaces
	// verbatim and prepends one more "    " to the *first line only*, so
	// naive pre-wrapping leaves continuation lines under-indented. We
	// wrap with an 8-space indent, then trim the first line back to 4
	// spaces; div1 restores it, and every line ends up aligned at 8.
	for _, name := range profileNames() {
		entry := fmt.Sprintf("%s — %s", name, profiles[name].description)
		wrapped := textwrap.Do(entry, 72, "        ")
		wrapped = "    " + strings.TrimPrefix(wrapped, "        ")
		desc = append(desc, wrapped)
	}
	desc = append(desc,
		"This is batteries-included convenience for the canonical star. For "+
			"asymmetric shapes beyond what the profiles express, or different "+
			"qdisc trees, call `npte netem apply` directly.",
		"With --dry-run, prints a round-trippable shell script to stdout instead "+
			"of executing anything. The output can be pasted into a shell (as root) "+
			"to reproduce the effect of a live run.",
	)
	usage.AddDescription(desc...)
	fset.Exit = env.Exit
	fset.Stderr = env.Stderr
	fset.Stdout = env.Stdout
	fset.UsagePrinter = usage
	fset.AutoHelp('h', "help", "Print this help text and exit.")
	var dryRun bool
	var profileName string
	fset.BoolVar(&dryRun, 'n', "dry-run", "Print the shell script instead of executing it.")
	fset.StringVar(&profileName, 0, "profile", "Named shaping profile to apply (empty = clear only).")
	fset.MinPositionalArgs = 0
	fset.MaxPositionalArgs = 0
	runtimex.PanicOnError0(fset.Parse(args))

	var prof profile
	if profileName != "" {
		var ok bool
		prof, ok = profiles[profileName]
		if !ok {
			logx.Error("npte star netem: unknown profile %q", profileName)
			logx.Error("npte star netem: available profiles: %v", profileNames())
			env.Exit(2)
			return nil
		}
	}

	self := selfPath("npte star netem")

	pass := func(argv ...string) []string {
		if dryRun {
			return append(argv, "-n")
		}
		return argv
	}

	if profileName == "" {
		logx.Details("npte: clear shaping on star access links")
	} else {
		logx.Details("npte: apply profile %q to star access links", profileName)
	}

	// Clear first unconditionally: makes the command idempotent and lets
	// the user switch between profiles without a separate teardown step.
	// `netem clear` is itself tolerant of an absent root qdisc, so this
	// is a no-op on a freshly-created star.
	runSelf(ctx, self, pass("netem", "clear", "router", "if-client")...)
	runSelf(ctx, self, pass("netem", "clear", "router", "if-server")...)

	if profileName == "" {
		return nil
	}

	dnArgs := applyArgs("router", "if-client", prof.downlink)
	upArgs := applyArgs("router", "if-server", prof.uplink)
	runSelf(ctx, self, pass(dnArgs...)...)
	runSelf(ctx, self, pass(upArgs...)...)

	return nil
}

// applyArgs returns an argv for `npte netem apply` that reproduces the
// given link shape on <ns>/<iface>. Flag order matches the canonical
// ordering used by netem/apply.go so --dry-run output is stable across
// scenarios and easy to diff.
func applyArgs(ns, iface string, shape linkShape) []string {
	argv := []string{"netem", "apply"}
	if shape.delay != "" {
		argv = append(argv, "--delay", shape.delay)
	}
	if shape.loss != "" {
		argv = append(argv, "--loss", shape.loss)
	}
	if shape.limit != "" {
		argv = append(argv, "--limit", shape.limit)
	}
	if shape.rate != "" {
		argv = append(argv, "--rate", shape.rate)
	}
	if shape.child != "" {
		argv = append(argv, "--child", shape.child)
	}
	argv = append(argv, ns, iface)
	return argv
}
