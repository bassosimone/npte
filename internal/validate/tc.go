// SPDX-License-Identifier: GPL-3.0-or-later

package validate

import (
	"fmt"
	"slices"
	"strings"
)

// AllowedChildQdiscs lists the qdisc kinds accepted by `npte netem
// apply --child`. The list is deliberately narrow because tc(8)
// autoloads the kernel module `sch_<kind>` via modprobe before
// attaching an unknown qdisc. Accepting an arbitrary kind would
// therefore expose a "load any sch_* kernel module on the host"
// primitive to whoever can invoke this command — a side effect that
// is not contained by the network namespace the qdisc is installed
// in.
//
// Today only `cake` is allowed: it is the one kind we surface knobs
// for (`--cake-bandwidth`) and the only kind any tutorial chapter or
// lab-netem profile invokes. Add a kind back the day a concrete
// use case shows up, alongside whatever per-kind flags it needs;
// kinds with no exposed knobs would only run at kernel defaults,
// which is meaningful for self-tuning AQMs but useless for FIFOs
// where the limit *is* the experiment.
//
// Exported so the CLI help text can enumerate the allowed kinds
// without copy-pasting the list.
var AllowedChildQdiscs = []string{
	"cake",
}

// ChildQdiscKind reports whether s is an accepted qdisc kind for
// `npte netem apply --child`. See [AllowedChildQdiscs] for rationale.
func ChildQdiscKind(s string) error {
	if !slices.Contains(AllowedChildQdiscs, s) {
		return fmt.Errorf("qdisc kind %q is not allowed for --child; permitted: %s",
			s, strings.Join(AllowedChildQdiscs, ", "))
	}
	return nil
}
