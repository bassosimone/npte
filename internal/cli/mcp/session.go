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
	"github.com/bassosimone/npte/internal/testable"
	"github.com/bassosimone/runtimex"
	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// sessionProc is the in-memory record for a single privileged child started
// via the MCP. The reaper goroutine writes [sessionProc.exitcode], closes
// [sessionProc.closers], and then closes [sessionProc.done] in that order;
// readers that observe done closed can therefore safely read exitcode without
// further synchronization.
type sessionProc struct {
	closers  *closepool.Pool
	cmd      *exec.Cmd
	done     chan struct{}
	exitcode int
	exitfile io.Writer
}

// sessionManager owns the table of in-flight processes started via the MCP.
// Methods [sessionManager.Start], [sessionManager.Wait], and
// [sessionManager.Kill] are the agent-facing tool handlers;
// [sessionManager.Cleanup] is the server shutdown sweep.
//
// All access to the process table is serialized by procsMu.
//
// Construct using [newSessionManager].
type sessionManager struct {
	env      *testable.Environ
	exePath  string
	procs    map[string]*sessionProc
	procsMu  sync.Mutex
	spoolDir string
}

// newSessionManager constructs a [sessionManager]. exePath is the absolute
// path to the npte binary the manager will re-exec under sudo. spoolDir is
// the absolute path under which per-process subdirectories are created; the
// caller must ensure it already exists on disk.
func newSessionManager(env *testable.Environ, exePath string, spoolDir string) *sessionManager {
	return &sessionManager{
		env:      env,
		exePath:  exePath,
		procs:    make(map[string]*sessionProc),
		procsMu:  sync.Mutex{},
		spoolDir: spoolDir,
	}
}

// startOutput is the output schema shared by every `start_*` MCP tool.
type startOutput struct {
	ProcDir string `json:"procDir" jsonschema:"Absolute path to the per-process spool directory. Contains argv.json (the full sudo-prefixed argv), stdout.txt (child stdout), stderr.txt (child stderr), and exitcode.txt (written when the process terminates). Read these files via your normal filesystem access; the MCP server only writes them and never reads them back."`
	ProcID  string `json:"procId" jsonschema:"Opaque UUIDv7 identifying the process. Pass to wait or kill."`
}

// startProc starts a privileged npte invocation as a background process and
// returns the [startOutput] the per-tool MCP handlers hand back to the agent.
//
// nptArgs is the argument tail starting from the top-level npte subcommand
// name (e.g., ["netem", "apply", "router", "if-server", "--rate", "10mbit"]).
// startProc itself prepends "/usr/bin/sudo", "-n", and the absolute npte
// path; per-tool handlers must not include those.
//
// On [exec.Cmd.Start] failure the proc is recorded as already-terminated
// with exit code 127 and a "fatal: ..." line in stderr.txt, so the agent's
// wait/kill flow remains uniform.
func (sm *sessionManager) startProc(nptArgs []string) (*startOutput, error) {
	// Make sure we close the files on failure
	pool := &closepool.Pool{}
	defer pool.Close()

	// Create subdirectory for the agent to get process information
	procID := runtimex.PanicOnError1(uuid.NewV7()).String()
	procDir := filepath.Join(sm.spoolDir, procID)
	if err := sm.env.MkdirAll(procDir, 0700); err != nil {
		return nil, err
	}

	// TODO(bassosimone): add mockable [os.OpenFile] inside [*testable.Environ]
	// and use the mock instead of using the real API

	// Create the required files
	const openFlags = os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	exitFile, err := os.OpenFile(filepath.Join(procDir, "exitcode.txt"), openFlags, 0600)
	if err != nil {
		return nil, err
	}
	pool.Add(exitFile)

	stdoutFile, err := os.OpenFile(filepath.Join(procDir, "stdout.txt"), openFlags, 0600)
	if err != nil {
		return nil, err
	}
	pool.Add(stdoutFile)

	stderrFile, err := os.OpenFile(filepath.Join(procDir, "stderr.txt"), openFlags, 0600)
	if err != nil {
		return nil, err
	}
	pool.Add(stderrFile)

	// Create the command to execute
	//
	// Use an explicit sudo path to avoid PATH hijacking
	args := append([]string{"/usr/bin/sudo", "-n", sm.exePath}, nptArgs...)
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
	err = cmd.Start()

	// Create process structure
	proc := &sessionProc{
		closers:  pool.Move(), // transfer ownership
		cmd:      cmd,
		done:     make(chan struct{}),
		exitcode: 0,
		exitfile: exitFile,
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
	sm.procsMu.Lock()
	sm.procs[procID] = proc
	sm.procsMu.Unlock()

	// On success arrange to reap the proc
	if err == nil {
		go sm.reaper(proc)
	}

	// Return proc information to the agent
	output := &startOutput{
		ProcID:  procID,
		ProcDir: procDir,
	}
	return output, nil
}

// reaper waits for the process to terminate and writes the exitcode file
// on disk. We close the [*sessionProc] done channel when done.
func (sm *sessionManager) reaper(proc *sessionProc) {
	// Wait and obtain the process exitcode
	var exitcode int
	err := proc.cmd.Wait()
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
	ProcID  string `json:"procId" jsonschema:"The procId returned by any start_* tool."`
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
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Access the process record
	sm.procsMu.Lock()
	p := sm.procs[input.ProcID]
	sm.procsMu.Unlock()
	if p == nil {
		return nil, nil, fmt.Errorf("no such process: %s", input.ProcID)
	}

	// Await process termination or timeout
	output := &waitOutput{}
	select {
	case <-p.done:
		// process terminated: remove from table
		sm.procsMu.Lock()
		delete(sm.procs, input.ProcID)
		sm.procsMu.Unlock()

		// ensure we record termination
		output.Terminated = true

		// grab exitcode after the sync barrier
		output.ExitCode = p.exitcode

	case <-ctx.Done():
		// timeout: nothing to do
	}

	return nil, output, nil
}

// killInput is the input schema for the `kill` MCP tool.
type killInput struct {
	ProcID string `json:"procId" jsonschema:"The procId returned by any start_* tool."`
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
	return nil, &killOutput{}, p.maybeKill(sigNo)

}

func (sp *sessionProc) maybeKill(sigNo syscall.Signal) error {
	// Avoid killing if it has already terminated
	select {
	case <-sp.done:
		return os.ErrProcessDone
	default:
	}

	// Kill the child process by PID
	runtimex.Assert(sp.cmd.Process != nil)
	return sp.cmd.Process.Signal(sigNo)
}

// Cleanup is the server shutdown sweep: it sends SIGTERM to every process
// still tracked in the manager and then waits for each one to terminate, or
// for ctx to expire. Called from [serveCleanup] once [mcp.Server.Run]
// returns.
func (sm *sessionManager) Cleanup(ctx context.Context) error {
	// Steal the list of currently running processes
	sm.procsMu.Lock()
	procs := slices.Collect(maps.Values(sm.procs))
	sm.procs = make(map[string]*sessionProc)
	sm.procsMu.Unlock()

	// Kill each currently running process
	for _, proc := range procs {
		proc.maybeKill(syscall.SIGTERM)
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
