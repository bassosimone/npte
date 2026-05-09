// SPDX-License-Identifier: GPL-3.0-or-later

package container

import (
	"context"
	"path/filepath"

	"github.com/bassosimone/npte/internal/logx"
	"github.com/bassosimone/npte/internal/subprocess"
	"github.com/bassosimone/npte/internal/testable"
	"github.com/bassosimone/npte/internal/validate"
	"github.com/bassosimone/runtimex"
	"github.com/bassosimone/vflag"
)

// createMain is the main of the `container create` subcommand.
func createMain(ctx context.Context, args []string) error {
	env := testable.Env

	fset := vflag.NewFlagSet("npte container create", vflag.ExitOnError)
	usage := vflag.NewDefaultUsagePrinter()
	usage.AddDescription(
		"Bootstraps a minimal Debian-family filesystem tree at <rootfs> using "+
			"debootstrap. Both <suite> (e.g. \"noble\", \"bookworm\") and <rootfs> "+
			"are required and map directly to the positional arguments of "+
			"debootstrap(8); this command is a thin wrapper that adds input "+
			"validation and dry-run support.",
		"The Ubuntu-shipped debootstrap package carries scripts for both Ubuntu "+
			"and Debian suites, so `noble` and `bookworm` both work from either "+
			"host. For non-Debian/Ubuntu derivatives (Kali, Devuan), drop to "+
			"`debootstrap` directly — the same applies to --include=, --variant=, "+
			"custom mirrors, and other debootstrap knobs.",
		"<rootfs> must be an absolute path; the target directory must not "+
			"exist or must be empty (debootstrap refuses to populate a non-"+
			"empty tree). For safety, pick a path that is fully root-owned "+
			"end-to-end — e.g. /var/lib/machines/<name>, the systemd-nspawn "+
			"convention. debootstrap runs as root and writes through whatever "+
			"path you hand it; an unprivileged owner along the path can "+
			"modify the tree while debootstrap is populating it.",
		"With --dry-run, prints a round-trippable shell script to stdout instead "+
			"of executing anything. The output can be pasted into a shell (as root) "+
			"to reproduce the effect of a live run. The script sets no shell "+
			"options of its own; wrap it (e.g. with `set -euxo pipefail`) "+
			"if you want fail-fast semantics.",
	)
	usage.PositionalArgumentsUsage = "<suite> <rootfs>"
	fset.Exit = env.Exit
	fset.Stderr = env.Stderr
	fset.Stdout = env.Stdout
	fset.UsagePrinter = usage
	fset.AutoHelp('h', "help", "Print this help text and exit.")
	var dryRun bool
	fset.BoolVar(&dryRun, 'n', "dry-run", "Print the shell script instead of executing it.")
	fset.MinPositionalArgs = 2
	fset.MaxPositionalArgs = 2
	runtimex.PanicOnError0(fset.Parse(args))

	suite := fset.Args()[0]
	rootfs := fset.Args()[1]

	if err := validate.DebootstrapSuite(suite); err != nil {
		logx.Error("npte container create: %s", err)
		env.Exit(2)
		return nil
	}
	if !filepath.IsAbs(rootfs) {
		logx.Error("npte container create: rootfs %q must be an absolute path", rootfs)
		env.Exit(2)
		return nil
	}
	logx.Details("npte: bootstrap %q into %s (this may take a while)", suite, rootfs)
	subprocess.MustRun(ctx, dryRun, "debootstrap", suite, rootfs)

	logx.Details("npte: filesystem tree ready at %s", rootfs)
	return nil
}
