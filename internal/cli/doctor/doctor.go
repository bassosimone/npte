// SPDX-License-Identifier: GPL-3.0-or-later

// Package doctor implements the doctor subcommand.
package doctor

import (
	"context"
	"fmt"
	"strings"

	"github.com/bassosimone/npte/internal/deps"
	"github.com/bassosimone/npte/internal/testable"
	"github.com/bassosimone/runtimex"
	"github.com/bassosimone/vflag"
	"github.com/charmbracelet/lipgloss"
)

// Main is the main of the doctor subcommand.
func Main(ctx context.Context, args []string) error {
	// Obtain the configured environment
	env := testable.Env

	// Parse command line flags
	fset := vflag.NewFlagSet("npte doctor", vflag.ExitOnError)
	usage := vflag.NewDefaultUsagePrinter()
	usage.AddDescription(
		"Checks that all required external commands are installed, " +
			"prints the path of the found commands or MISSING with the " +
			"Debian package name, suggests how to install missing packages.",
	)
	fset.Exit = env.Exit
	fset.Stderr = env.Stderr
	fset.Stdout = env.Stdout
	fset.UsagePrinter = usage
	fset.AutoHelp('h', "help", "Print this help text and exit.")
	fset.MaxPositionalArgs = 0
	runtimex.PanicOnError0(fset.Parse(args)) // cannot fail: using vflag.ExitOnError

	red := env.LogRenderer.NewStyle().Foreground(lipgloss.Color("1"))
	green := env.LogRenderer.NewStyle().Foreground(lipgloss.Color("2"))

	var missing []string
	for _, dep := range deps.All {
		path, err := deps.LookPath(dep.Binary)
		if err != nil {
			status := red.Render(fmt.Sprintf("MISSING (%s)", dep.Pkg))
			fmt.Fprintf(env.Stdout, "checking for %s... %s\n", dep.Binary, status)
			missing = append(missing, dep.Pkg)
			continue
		}
		fmt.Fprintf(env.Stdout, "checking for %s... %s\n", dep.Binary, green.Render(path))
	}

	// sudo is checked at its fixed path with a stat, NOT via
	// deps.LookPath: it is deliberately excluded from deps.All, and
	// `npte mcp serve` execs exactly this path. See the deps.SudoPath
	// invariant — do not "simplify" this into a deps.All entry.
	if _, err := env.Lstat(deps.SudoPath); err != nil {
		status := red.Render("MISSING (sudo)")
		fmt.Fprintf(env.Stdout, "checking for %s... %s\n", deps.SudoPath, status)
		missing = append(missing, "sudo")
	} else {
		fmt.Fprintf(env.Stdout, "checking for %s... %s\n", deps.SudoPath, green.Render(deps.SudoPath))
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
