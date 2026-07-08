// SPDX-License-Identifier: GPL-3.0-or-later

package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/bassosimone/closepool"
	"github.com/bassosimone/npte/internal/deps"
	"github.com/bassosimone/npte/internal/testable"
	"github.com/bassosimone/runtimex"
	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// sessionProc is the in-memory record for a single privileged child started
// via the MCP. The reaper goroutine writes [sessionProc.exitcode], closes
// [sessionProc.closers], and then closes [sessionProc.done] in that order;
// readers that observe done closed can therefore safely read the process
// exitcode without further synchronization.
type sessionProc struct {
	closers  *closepool.Pool
	cmd      *exec.Cmd
	done     chan struct{}
	exitcode int
	exitfile io.Writer
	procDir  string
	procID   string
}

// sessionManager owns the table of in-flight processes
// started via the MCP.
//
// Methods [sessionManager.Start], [sessionManager.Wait], and
// [sessionManager.Kill] are the agent-facing tool handlers;
// [sessionManager.Cleanup] is the server shutdown sweep.
//
// All access to the process table is serialized by procsMu.
//
// Construct using [newSessionManager].
type sessionManager struct {
	env        *testable.Environ
	exePath    string
	procs      map[string]*sessionProc
	procsMu    sync.Mutex
	sessionDir string
}

// newSessionManager constructs a [sessionManager]. exePath is the absolute
// path to the npte binary the manager will re-exec under sudo. sessionDir
// is the absolute path to the current MCP session's directory, under which
// per-process subdirectories are created; the caller must ensure it
// already exists on disk.
func newSessionManager(env *testable.Environ, exePath string, sessionDir string) *sessionManager {
	return &sessionManager{
		env:        env,
		exePath:    exePath,
		procs:      make(map[string]*sessionProc),
		procsMu:    sync.Mutex{},
		sessionDir: sessionDir,
	}
}

// runOutput is the output schema for MCP tools that run a process
// to completion synchronously (as opposed to starting it in the
// background and requiring a separate wait call).
type runOutput struct {
	ProcDir    string `json:"procDir" jsonschema:"Absolute path to the per-process directory under the current MCP session. Contains argv.json (the full sudo-prefixed argv), stdout.txt (child stdout), stderr.txt (child stderr), and exitcode.txt (written when the process terminates). Read these files via your normal filesystem access; the MCP server only writes them and never reads them back."`
	ProcID     string `json:"procId" jsonschema:"Opaque UUIDv7 identifying the process within the session."`
	ExitCode   int    `json:"exitCode" jsonschema:"Exit code of the terminated process. Meaningful only when terminated is true; ignore otherwise."`
	Terminated bool   `json:"terminated" jsonschema:"True if the process terminated within the timeout; false if the wait timed out and the process is still running. Acts as the validity flag for exitCode."`
}

// runStep is one sub-step within a multi-step MCP tool invocation.
type runStep struct {
	Step       string `json:"step" jsonschema:"Human-readable label identifying this sub-step."`
	ProcDir    string `json:"procDir" jsonschema:"Absolute path to the per-process directory under the current MCP session. Contains argv.json, stdout.txt, stderr.txt, and exitcode.txt."`
	ProcID     string `json:"procId" jsonschema:"Opaque UUIDv7 identifying the process within the session."`
	ExitCode   int    `json:"exitCode" jsonschema:"Exit code of the terminated process. Meaningful only when terminated is true."`
	Terminated bool   `json:"terminated" jsonschema:"True if the process terminated within the timeout; false otherwise."`
}

func (ro *runOutput) toStep(label string) *runStep {
	return &runStep{
		Step:       label,
		ProcDir:    ro.ProcDir,
		ProcID:     ro.ProcID,
		ExitCode:   ro.ExitCode,
		Terminated: ro.Terminated,
	}
}

// multiRunOutput is the output schema for MCP tools that run multiple
// npte invocations sequentially and return the result of each step.
type multiRunOutput struct {
	Steps []*runStep `json:"steps" jsonschema:"Ordered list of sub-steps executed by this tool call. Each entry corresponds to one privileged npte invocation."`
}

// runProc starts a privileged npte invocation, waits for it to
// terminate (up to waitTimeout), and returns the combined result.
// If the process does not finish in time, it is killed and given
// a short grace period before the result is returned.
func (sm *sessionManager) runProc(
	ctx context.Context, waitTimeout time.Duration, nptArgs []string) (*runOutput, error) {
	// Start the process using startProc
	proc, err := sm.startProc(nptArgs)
	if err != nil {
		return nil, err
	}

	// Wait for process termination
	wait := sm.WaitSessionProc(ctx, proc, waitTimeout)

	// Handle the case where the process did not finish running
	// by killing the process and moving on. Note that our options
	// for escalating the killing are very limited here.
	if !wait.Terminated {
		sm.MaybeKill(proc, syscall.SIGINT)
		sm.MaybeKill(proc, syscall.SIGTERM)
		const extraWaitTimeout = 5 * time.Second
		wait = sm.WaitSessionProc(ctx, proc, extraWaitTimeout)
	}

	// Return output to the agent
	runtimex.Assert(wait != nil)
	output := &runOutput{
		ProcDir:    proc.procDir,
		ProcID:     proc.procID,
		ExitCode:   wait.ExitCode,
		Terminated: wait.Terminated,
	}
	return output, nil
}

// startOutput is the output schema for the `start_command` MCP tool,
// which starts a background process requiring explicit wait/kill.
type startOutput struct {
	ProcDir string `json:"procDir" jsonschema:"Absolute path to the per-process directory under the current MCP session. Contains argv.json (the full sudo-prefixed argv), stdout.txt (child stdout), stderr.txt (child stderr), and exitcode.txt (written when the process terminates). Read these files via your normal filesystem access; the MCP server only writes them and never reads them back."`
	ProcID  string `json:"procId" jsonschema:"Opaque UUIDv7 identifying the process. Pass to wait or kill to observe termination or interrupt the process."`
}

// startProc starts a privileged npte invocation as a background process and
// returns the [*sessionProc] representing the running process.
//
// nptArgs is the argument tail starting from the top-level npte subcommand
// name (e.g., ["netem", "apply", "router", "if-server", "--rate", "10mbit"]).
// startProc itself prepends "/usr/bin/sudo", "-n", and the absolute npte
// path; per-tool handlers must not include those.
//
// On [exec.Cmd.Start] failure the proc is recorded as already-terminated
// with exit code 127 and a "fatal: ..." line in stderr.txt, so the agent's
// wait/kill flow remains uniform.
func (sm *sessionManager) startProc(nptArgs []string) (*sessionProc, error) {
	// Centralize closing all the open files
	pool := &closepool.Pool{}
	defer pool.Close()

	// Create subdirectory for the agent to get process information
	procID := runtimex.PanicOnError1(uuid.NewV7()).String()
	procDir := filepath.Join(sm.sessionDir, procID)
	if err := sm.env.MkdirAll(procDir, 0700); err != nil {
		return nil, err
	}

	// Create the required files
	const openFlags = os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	exitFile, err := sm.env.OpenFile(filepath.Join(procDir, "exitcode.txt"), openFlags, 0600)
	if err != nil {
		return nil, err
	}
	pool.Add(exitFile)

	stdoutFile, err := sm.env.OpenFile(filepath.Join(procDir, "stdout.txt"), openFlags, 0600)
	if err != nil {
		return nil, err
	}
	pool.Add(stdoutFile)

	stderrFile, err := sm.env.OpenFile(filepath.Join(procDir, "stderr.txt"), openFlags, 0600)
	if err != nil {
		return nil, err
	}
	pool.Add(stderrFile)

	// Create the command to execute
	//
	// SECURITY: sudo is exec'd at a fixed path, never resolved via
	// PATH and never via deps.LookPath — sudo is deliberately excluded
	// from deps.All. See the deps.SudoPath invariant for the full
	// rationale (PATH hijacking of the trust bridge; keeping sudo out
	// of the subprocess allowlist).
	args := append([]string{deps.SudoPath, "-n", sm.exePath}, nptArgs...)
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdout = stdoutFile
	cmd.Stderr = stderrFile

	// Write the command line
	argvFilePath := filepath.Join(procDir, "argv.json")
	rawArgv := runtimex.PanicOnError1(json.Marshal(args)) // Cannot fail
	if err := sm.env.WriteFile(argvFilePath, rawArgv, 0600); err != nil {
		return nil, err
	}

	// Start the background command
	err = sm.env.StartCommand(cmd)

	// Create process structure
	proc := &sessionProc{
		closers:  pool.Move(), // transfer ownership
		cmd:      cmd,
		done:     make(chan struct{}),
		exitcode: 0,
		exitfile: exitFile,
		procDir:  procDir,
		procID:   procID,
	}

	// On failure simulate wait's execution
	if err != nil {
		fmt.Fprintf(stderrFile, "fatal: %s\n", err.Error())
		fmt.Fprintf(exitFile, "127\n")
		proc.exitcode = 127
		proc.closers.Close()
		close(proc.done)
		// fallthrough to fake a successful start
	}

	// Remember the proc
	//
	// TODO(bassosimone): this insert races with [sessionManager.Cleanup]:
	// a tool handler still executing when server.Run returns can insert
	// here after Cleanup has already swept the table, so that child is
	// never signaled at shutdown and survives the server as an orphaned
	// root process. Whether the window is actually reachable depends on
	// whether the SDK drains in-flight requests before Run returns.
	// Consider a shutdown flag set by Cleanup under procsMu: once set,
	// startProc should refuse to start new children.
	sm.procsMu.Lock()
	sm.procs[procID] = proc
	sm.procsMu.Unlock()

	// On success arrange to reap the proc
	if err == nil {
		go sm.reaper(proc)
	}

	// Return the running process
	return proc, nil
}

// reaper waits for the process to terminate and writes the exitcode file
// on disk. We close the [*sessionProc] done channel when done.
func (sm *sessionManager) reaper(proc *sessionProc) {
	// Wait and obtain the process exitcode
	var exitcode int
	err := sm.env.WaitCommand(proc.cmd)
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			exitcode = ee.ExitCode()
		} else {
			exitcode = 255
		}
	} else {
		exitcode = 0
	}

	// Write the exitcode to the proper file ignoring
	// errors because we cannot do much about them
	_, _ = fmt.Fprintf(proc.exitfile, "%d\n", exitcode)

	// Close all the files we've opened also ignoring
	// errors because we cannot do much about them
	_ = proc.closers.Close()

	// Write the exitcode before the barrier
	proc.exitcode = exitcode

	// Synchronize with waiters
	close(proc.done)
}

// waitInput is the input schema for the `wait` MCP tool.
type waitInput struct {
	ProcID  string `json:"procId" jsonschema:"The procId returned by start_command."`
	Timeout string `json:"timeout" jsonschema:"Maximum time to wait, as a Go duration string (e.g., \"30s\", \"1m500ms\", \"2h\"). The call returns as soon as the process terminates or this duration elapses, whichever comes first."`
}

// waitOutput is the output schema for the `wait` MCP tool.
type waitOutput struct {
	ExitCode   int  `json:"exitCode" jsonschema:"Exit code of the terminated process. Meaningful only when terminated is true; ignore otherwise."`
	Terminated bool `json:"terminated" jsonschema:"True if the process terminated within the timeout; false if the wait timed out and the process is still running. Acts as the validity flag for exitCode."`
}

// Wait is the MCP handler for the `wait` tool. See the tool's registration
// in [serveMain] for the agent-facing description.
func (sm *sessionManager) Wait(ctx context.Context, req *mcp.CallToolRequest,
	input *waitInput) (*mcp.CallToolResult, *waitOutput, error) {
	// Start by parsing the timeout
	timeout, err := time.ParseDuration(input.Timeout)
	if err != nil {
		return nil, nil, err
	}

	// Access the process record
	sm.procsMu.Lock()
	p := sm.procs[input.ProcID]
	sm.procsMu.Unlock()
	if p == nil {
		return nil, nil, fmt.Errorf("no such process: %s", input.ProcID)
	}

	return nil, sm.WaitSessionProc(ctx, p, timeout), nil
}

// WaitSessionProc waits for the [*sessionProc] to terminate and removes
// it from the [*sessionManager] internal process tracking table.
//
// This method always returns a non-nil [*waitOutput] struct.
func (sm *sessionManager) WaitSessionProc(
	ctx context.Context, p *sessionProc, timeout time.Duration) *waitOutput {
	// Arm the context to bound the total wait time
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Await process termination or timeout
	output := &waitOutput{}
	select {
	case <-p.done:
		// process terminated: remove from table
		sm.procsMu.Lock()
		delete(sm.procs, p.procID)
		sm.procsMu.Unlock()

		// ensure we record termination
		output.Terminated = true

		// grab exitcode after the sync barrier
		output.ExitCode = p.exitcode

	case <-ctx.Done():
		// timeout: nothing to do
	}

	return output
}

// killInput is the input schema for the `kill` MCP tool.
type killInput struct {
	ProcID string `json:"procId" jsonschema:"The procId returned by start_command."`
	Signal string `json:"signal" jsonschema:"Signal name to deliver. Currently only \"INT\" (or \"SIGINT\") is accepted. SIGKILL is deliberately unsupported because sudo cannot relay it to the privileged child."`
}

// killOutput is the (empty) output schema for the `kill` MCP tool.
type killOutput struct{}

// Kill is the MCP handler for the `kill` tool. See the tool's registration
// in [serveMain] for the agent-facing description.
func (sm *sessionManager) Kill(ctx context.Context, req *mcp.CallToolRequest,
	input *killInput) (*mcp.CallToolResult, *killOutput, error) {
	// Normalize the signal name
	sigName := strings.TrimPrefix(input.Signal, "SIG")
	var sigNo syscall.Signal
	switch sigName {
	case "INT":
		sigNo = syscall.SIGINT
	default:
		return nil, nil, fmt.Errorf("unsupported signal: %q", sigName)
	}

	// Access the process record
	sm.procsMu.Lock()
	p := sm.procs[input.ProcID]
	sm.procsMu.Unlock()
	if p == nil {
		return nil, nil, fmt.Errorf("no such process: %s", input.ProcID)
	}

	// Kill the child process by PID
	return nil, &killOutput{}, sm.MaybeKill(p, sigNo)

}

// MaybeKill kills the process if running and otherwise does nothing.
//
// Signaling the root sudo child from this unprivileged process works
// because the sudo front-end keeps its real uid equal to the invoking
// user (verified empirically with sudo-rs, 2026-07). Caveat: when sudo
// has a controlling tty it runs a pty monitor that drops relayed
// SIGINT/SIGQUIT ("keyboard signals"); the MCP server has no tty, so
// SIGINT goes through here. SIGTERM is relayed unconditionally either
// way, which keeps the runProc escalation and Cleanup robust.
func (sm *sessionManager) MaybeKill(sp *sessionProc, sigNo syscall.Signal) error {
	// Avoid killing if it has already terminated
	select {
	case <-sp.done:
		return os.ErrProcessDone
	default:
	}

	// Kill the child process by PID
	return sm.env.ProcessSignal(sp.cmd, sigNo)
}

// Cleanup is the server shutdown sweep: it sends SIGTERM to every process
// still tracked in the manager and then waits for each one to terminate, or
// for the context to expire.
func (sm *sessionManager) Cleanup(ctx context.Context) error {
	// Steal the list of currently running processes
	sm.procsMu.Lock()
	procs := slices.Collect(maps.Values(sm.procs))
	sm.procs = make(map[string]*sessionProc)
	sm.procsMu.Unlock()

	// Kill each currently running process
	for _, proc := range procs {
		sm.MaybeKill(proc, syscall.SIGTERM)
	}

	// Wait for each process to terminate or context to expire
	for _, proc := range procs {
		select {
		case <-proc.done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}
