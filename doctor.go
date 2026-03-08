// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/bassosimone/runtimex"
	"github.com/bassosimone/vflag"
	"github.com/charmbracelet/lipgloss"
)

// dependency describes an external command and its Debian package.
type dependency struct {
	binary string
	pkg    string
}

var dependencies = []dependency{
	{"ip", "iproute2"},
	{"tc", "iproute2"},
	{"iptables", "iptables"},
	{"sysctl", "procps"},
	{"systemd-run", "systemd"},
	{"systemd-nspawn", "systemd-container"},
	{"debootstrap", "debootstrap"},
}

func doctorMain(ctx context.Context, args []string) error {
	// Parse command line flags
	fset := vflag.NewFlagSet("npte doctor", vflag.ExitOnError)
	usage := vflag.NewDefaultUsagePrinter()
	usage.AddDescription(
		"Check that all required external commands are installed. " +
			"Reports the path of each found command or MISSING with the " +
			"Debian package name. Suggests an apt install command for any missing packages.",
	)
	fset.UsagePrinter = usage
	fset.AutoHelp('h', "help", "Print this help text and exit.")
	fset.MaxPositionalArgs = 0
	runtimex.PanicOnError0(fset.Parse(args))

	red := env.LogRenderer.NewStyle().Foreground(lipgloss.Color("1"))
	green := env.LogRenderer.NewStyle().Foreground(lipgloss.Color("2"))

	var missing []string
	for _, dep := range dependencies {
		path, err := env.LookPath(dep.binary)
		if err != nil {
			status := red.Render(fmt.Sprintf("MISSING (%s)", dep.pkg))
			fmt.Fprintf(env.Stdout, "checking for %s... %s\n", dep.binary, status)
			missing = append(missing, dep.pkg)
		} else {
			fmt.Fprintf(env.Stdout, "checking for %s... %s\n", dep.binary, green.Render(path))
		}
	}

	if len(missing) > 0 {
		// Deduplicate (e.g., iproute2 appears twice)
		seen := make(map[string]bool)
		var unique []string
		for _, pkg := range missing {
			if !seen[pkg] {
				seen[pkg] = true
				unique = append(unique, pkg)
			}
		}
		fix := red.Render("Fix by running: sudo apt install " + strings.Join(unique, " "))
		fmt.Fprintf(env.Stdout, "\n%s\n", fix)
		env.Exit(1)
	}

	return nil
}
