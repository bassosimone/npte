// SPDX-License-Identifier: GPL-3.0-or-later

// Package netem implements the netem subcommand.
package netem

import (
	"context"

	"github.com/bassosimone/npte/internal/testable"
	"github.com/bassosimone/vclip"
	"github.com/bassosimone/vflag"
)

// Main is the main of the netem subcommand.
func Main(ctx context.Context, args []string) error {
	env := testable.Env

	disp := vclip.NewDispatcherCommand("npte netem", vflag.ExitOnError)
	disp.Exit = env.Exit
	disp.Stderr = env.Stderr
	disp.Stdout = env.Stdout
	disp.AddDescription(
		"Apply or clear tc/netem traffic shaping on an interface inside a "+
			"namespace. The root qdisc is always `netem` with handle 1:, so a "+
			"child qdisc can optionally be attached at parent 1: for AQM "+
			"experiments.",
		"Shaping is per-direction and per-interface: the qdisc affects packets "+
			"egressing <if> inside <ns>. An asymmetric link is two `apply` "+
			"calls on the two ns+iface endpoints of the veth pair.",
		"This command commits to `root netem` plus an optional one-level "+
			"child. Anything else (TBF-as-root, HTB classes, hierarchical "+
			"trees) is out of scope; use `npte netns run --user root <ns> "+
			"tc ...` for those cases.",
	)
	disp.AddCommand("apply", vclip.CommandFunc(applyMain), "Apply netem shaping to an interface.")
	disp.AddCommand("clear", vclip.CommandFunc(clearMain), "Remove netem shaping from an interface.")

	return disp.Main(ctx, args)
}
