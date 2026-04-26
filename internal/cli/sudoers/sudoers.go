// SPDX-License-Identifier: GPL-3.0-or-later

// Package sudoers implements the sudoers subcommand.
package sudoers

import (
	"context"
	"fmt"

	"github.com/bassosimone/npte/internal/logx"
	"github.com/bassosimone/npte/internal/testable"
	"github.com/bassosimone/npte/internal/validate"
	"github.com/bassosimone/runtimex"
	"github.com/bassosimone/vflag"
)

// installPath is the absolute path where npte must be installed for
// the emitted sudoers snippet to take effect. Sudoers Cmnd lookups
// are by absolute path, so the snippet binds to a single, fixed path
// rather than to whatever os.Executable() happens to return when
// `npte sudoers` is invoked. If the user's binary lives somewhere
// else, the right answer is to install it here, not to teach the
// snippet about per-host paths.
const installPath = "/usr/local/sbin/npte"

// Main is the main of the sudoers subcommand.
func Main(ctx context.Context, args []string) error {
	env := testable.Env

	fset := vflag.NewFlagSet("npte sudoers", vflag.ExitOnError)
	usage := vflag.NewDefaultUsagePrinter()
	usage.AddDescription(
		"Prints a sudoers snippet that allowlists the npte netns and "+
			"netem subcommands for the invoking user, with NOPASSWD. "+
			"Other npte subcommands (gateway, star, container, "+
			"tutorial, ...) are not covered and continue to prompt for "+
			"the sudo password.",
		"The snippet binds the allowlist to "+installPath+", which is "+
			"where `sudo make install` from the npte source tree places "+
			"the binary. If your npte binary lives elsewhere, install it "+
			"at that path before activating the snippet — sudoers Cmnd "+
			"lookups are by absolute path and will not match a different "+
			"location.",
		"This command does not write any files; it only prints. The "+
			"output is itself a valid sudoers fragment (comments + "+
			"rules) and is designed to be pasted into a sudoers file "+
			"of your choice via visudo. Run this command without sudo: "+
			"it reads $USER to identify the target user, and refuses "+
			"if invoked as root.",
	)
	fset.Exit = env.Exit
	fset.Stderr = env.Stderr
	fset.Stdout = env.Stdout
	fset.UsagePrinter = usage
	fset.AutoHelp('h', "help", "Print this help text and exit.")
	fset.MaxPositionalArgs = 0
	runtimex.PanicOnError0(fset.Parse(args))

	if env.Geteuid() == 0 {
		logx.Error("npte sudoers: adding a sudoers snippet for root does not make sense")
		logx.Error("npte sudoers: run this command as the user who should be allowlisted")
		env.Exit(2)
		return nil
	}

	user := env.Getenv("USER")
	if user == "" {
		logx.Error("npte sudoers: $USER is not set; cannot determine which user to allowlist")
		env.Exit(2)
		return nil
	}
	if err := validate.Username(user); err != nil {
		logx.Error("npte sudoers: $USER: %s", err)
		env.Exit(2)
		return nil
	}

	fmt.Fprintf(env.Stdout, snippet, user)
	return nil
}

// snippet is the printf format for the emitted sudoers fragment.
//
// The whole output is itself a valid sudoers fragment: leading and
// trailing blank lines, '#' comments, and two NOPASSWD rules. It is
// designed to be pasted as-is into /etc/sudoers via `sudo visudo`.
//
// We recommend the main /etc/sudoers file rather than a drop-in
// under /etc/sudoers.d/ because `sudo visudo -f /etc/sudoers.d/...`
// inspects only the file being edited and prints an alarming "you
// have removed your ability to run sudo visudo again" warning when
// the file does not itself grant visudo — even though the merged
// runtime config (which is what sudo actually evaluates) still does.
// The warning is a false positive, but it is scary enough that
// recommending the drop-in path is bad DX.
//
// Cmnd_Alias and Defaults! were considered and dropped: with two
// rules an alias adds indirection without reuse, and `env_reset`
// plus a `secure_path` are sudo's defaults on every mainstream
// distro, so per-rule overrides would be belt-and-suspenders
// documentation rather than an enforced invariant.
const snippet = `
# This snippet allows running the following commands without a password:
#
# - ` + installPath + ` netns *
# - ` + installPath + ` netem *
#
# The user who is granted permission is the one who invoked
# the 'npte sudoers' command.

%[1]s ALL=(root) NOPASSWD: ` + installPath + ` netns *
%[1]s ALL=(root) NOPASSWD: ` + installPath + ` netem *

# Install this snippet by pasting it into /etc/sudoers via:
#
#     sudo visudo
#
# visudo validates the snippet's syntax before activating the change.

`
