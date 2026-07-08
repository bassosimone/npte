// SPDX-License-Identifier: GPL-3.0-or-later

package subprocess

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"

	"github.com/bassosimone/closepool"
	"github.com/bassosimone/npte/internal/deps"
	"github.com/bassosimone/npte/internal/logx"
	"github.com/bassosimone/npte/internal/testable"
	"github.com/bassosimone/runtimex"
	"github.com/kballard/go-shellquote"
)

// Pipeline runs a shell-style pipeline of commands wired with pipes.
// Each stage is an argv slice whose first element must be a command name
// in [deps.All]. The first stage's stdin is not connected; the last
// stage's stdout goes to env.Stdout; every stage's stderr goes to
// env.Stderr.
//
// When dryRun is true, Pipeline prints a single shell line with the
// stages joined by " | ".
//
// In live mode, all stages are started concurrently and connected via
// [io.Pipe] instances. When a stage exits, its downstream pipe writer is
// closed (thus sending EOF to the next stage) while its upstream reader
// is closed (thus sending broken-pipe to the previous stage). The returned
// error joins stage errors using [errors.Join].
//
// This function panics if any stage contains zero entries.
//
// The combined pipeline stdout is the stdout exposed by [*testable.Environ]
// while the combined pipeline stdin is /dev/null.
func Pipeline(ctx context.Context, dryRun bool, stages ...[]string) error {
	// Handle the pipeline containing no stages.
	env := testable.Env
	total := len(stages)
	if total <= 0 {
		return fmt.Errorf("subprocess: pipeline has no stages")
	}

	// Build the command line that would be executed.
	parts := make([]string, total)
	for idx, argv := range stages {
		runtimex.Assert(len(argv) > 0) // check before the `--dry-run` branch
		parts[idx] = shellquote.Join(argv...)
	}
	line := strings.Join(parts, " | ")

	// Just print the command line if `--dry-run`.
	if dryRun {
		_, err := fmt.Fprintf(env.Stdout, "%s\n", line)
		return err
	}

	// Setup all the commands to execute.
	cmds := make([]*exec.Cmd, total)
	for idx, argv := range stages {
		path, err := deps.LookPath(argv[0])
		if err != nil {
			return err
		}
		cmds[idx] = exec.CommandContext(ctx, path, argv[1:]...)
		cmds[idx].Stderr = env.Stderr
	}

	// The final command's stdout is the pipeline stdout.
	cmds[total-1].Stdout = env.Stdout

	// Connect stdout at stage IDX with stdin at stage IDX+1.
	var (
		closers = &closepool.Pool{}
		stdins  = make([]*io.PipeReader, total-1)
		stdouts = make([]*io.PipeWriter, total-1)
	)
	for idx := 0; idx < total-1; idx++ {
		pr, pw := io.Pipe()
		stdins[idx] = pr
		stdouts[idx] = pw
		cmds[idx].Stdout = pw
		cmds[idx+1].Stdin = pr
		closers.Add(pr)
		closers.Add(pw)
	}
	defer closers.Close()

	// Log the pipeline that we're executing.
	logx.Command("%s", line)

	// Attempt to start each stage.
	for idx, cmd := range cmds {
		if err := cmd.Start(); err != nil {
			// Explicitly close pipeline stages
			closers.Close()

			// Explicitly kill and wait each process
			for jdx := range idx {
				p := cmds[jdx]
				runtimex.Assert(p.Process != nil)
				p.Process.Kill()
				p.Wait()
			}

			// Return the error
			return fmt.Errorf("subprocess: pipeline stage %d (%s): start: %w", idx, stages[idx][0], err)
		}
	}

	// Block until all stages have terminated.
	var (
		errs = make([]error, total)
		wg   sync.WaitGroup
	)
	for idx := range total {
		wg.Go(func() {
			errs[idx] = cmds[idx].Wait()
			if idx < len(stdouts) {
				stdouts[idx].Close() // close stage stdout (-> EOF)
			}
			if idx > 0 {
				stdins[idx-1].Close() // close stage stdin (-> EPIPE)
			}
		})
	}
	wg.Wait()

	// Assemble resulting error or nil if there's no error.
	//
	// TODO(bassosimone): unlike the start path above, Wait errors are
	// joined bare, so a failure surfaces as an anonymous "exit status N"
	// with no hint of which stage produced it (e.g. grep exiting 1 when
	// it selects no lines looks the same as iptables-restore failing).
	// Consider wrapping each error with the stage index and argv0 like
	// the start-failure message does.
	return errors.Join(errs...)
}

// MustPipeline is like [Pipeline] but logs and exits on failure.
func MustPipeline(ctx context.Context, dryRun bool, stages ...[]string) {
	testable.Env.LogFatalOnError0(Pipeline(ctx, dryRun, stages...))
}
