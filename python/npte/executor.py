"""Internal module for invoking ``sudo npte`` safely."""

# SPDX-License-Identifier: GPL-3.0-or-later

import dataclasses
import json
import os
import shlex
import signal
import subprocess
import sys
import uuid
from typing import Protocol


@dataclasses.dataclass(frozen=True)
class Argv:
    """Structured representation of an ``npte`` subcommand invocation."""

    subcommand: list[str]
    flags: dict[str, str] = dataclasses.field(default_factory=dict)
    positionals: list[str] = dataclasses.field(default_factory=list)


class PopenProtocol(Protocol):
    """Structural protocol exposing the ``subprocess.Popen`` surface we use."""

    def poll(self) -> int | None: ...
    def wait(self, timeout: float | None = None) -> int: ...
    def send_signal(self, sig: int) -> None: ...


class TerminatedProcess:
    """A process that has finished executing."""

    def __init__(self, *, proc_dir: str, exitcode: int) -> None:
        self.proc_dir = proc_dir
        self.exitcode = exitcode

    def read_stdout_file(self) -> bytes:
        """Read and return the content of the stdout file."""
        with open(os.path.join(self.proc_dir, "stdout.txt"), "rb") as filep:
            return filep.read()

    def read_stderr_file(self) -> bytes:
        """Read and return the content of the stderr file."""
        with open(os.path.join(self.proc_dir, "stderr.txt"), "rb") as filep:
            return filep.read()


class RunningProcess:
    """A process that is currently executing."""

    def __init__(
        self,
        *,
        proc_dir: str,
        proc: PopenProtocol,
        files: list,
    ) -> None:
        self.proc_dir = proc_dir
        self._proc = proc
        self._files = files

    def is_running(self) -> bool:
        """Check whether the process is still running."""
        return self._proc.poll() is None

    def wait(self, *, timeout: float | None = None) -> TerminatedProcess:
        """
        Wait for the process to finish and return a ``TerminatedProcess``.

        Raises ``subprocess.TimeoutExpired`` if the process does not
        terminate within ``timeout`` seconds; in that case the process
        keeps running and you can ``kill`` and ``wait`` again.
        """
        # Gather the exitcode.
        exitcode = self._proc.wait(timeout=timeout)

        # Make sure we close stdout/stderr and possibly other open files.
        for filep in self._files:
            filep.close()
        self._files = []

        # Write the exitcode file.
        with open(os.path.join(self.proc_dir, "exitcode.txt"), "w") as filep:
            filep.write(str(exitcode))

        # Return a ``TerminatedProcess`` to the caller.
        return TerminatedProcess(proc_dir=self.proc_dir, exitcode=exitcode)

    def kill(self) -> None:
        """
        Send SIGINT to the underlying process.

        We signal the sudo front-end, which relays a kill(2)-sent SIGINT
        to the privileged child with or without a controlling tty
        (verified empirically with sudo-rs, 2026-07). SIGKILL would be
        pointless: sudo cannot relay it, so it would only kill the
        front-end and orphan the root-owned chain.
        """
        self._proc.send_signal(signal.SIGINT)


class NpteSudoExecutor:
    """
    Safely execute short-lived and long-running processes using ``sudo npte``.

    Each invocation writes argv.json, stdout.txt, stderr.txt, and
    exitcode.txt under ``.npte/sessions/<session>/<process>/`` for
    later inspection.
    """

    def __init__(self, npte: str, *, sessions_dir: str | None = None) -> None:
        self._npte = npte
        base = (
            sessions_dir
            if sessions_dir is not None
            else os.path.join(".npte", "sessions")
        )
        self._session_dir = os.path.join(base, str(uuid.uuid7()))
        os.makedirs(self._session_dir)

    def run_sudo_npte(
        self,
        argv: Argv,
        *,
        timeout: float | None = None,
        check: bool = True,
    ) -> TerminatedProcess:
        """
        Run a short-lived process using ``sudo npte``.

        Raises ``subprocess.CalledProcessError`` if ``check`` is true
        and the command exits with a nonzero code.

        Caveat: on timeout, subprocess.run sends SIGKILL to sudo, which
        cannot relay it. Cleanup of the privileged child then depends on
        sudo's pty monitor (interactive tty) or the service manager's
        cgroup sweep (systemd-run); in bare tty-less environments (e.g.,
        cron) the root-owned chain survives and keeps its namespace alive.
        """
        # Create the argument vector and the process directory.
        full_argv = self._make_argv(argv)
        print(f"+ {shlex.join(full_argv)}", file=sys.stderr)
        proc_dir = self._make_proc_dir_and_write_argv_json(full_argv)

        # Run the process possibly handling the timeout case.
        try:
            result = subprocess.run(
                full_argv,
                stdin=subprocess.DEVNULL,
                capture_output=True,
                timeout=timeout,
            )
        except subprocess.TimeoutExpired as exc:
            with open(os.path.join(proc_dir, "stdout.txt"), "wb") as filep:
                filep.write(exc.stdout or b"")
            with open(os.path.join(proc_dir, "stderr.txt"), "wb") as filep:
                filep.write(exc.stderr or b"")
            with open(os.path.join(proc_dir, "exitcode.txt"), "w") as filep:
                filep.write("timeout")
            raise

        # Write the stdout/stderr and the exitcode files.
        with open(os.path.join(proc_dir, "stdout.txt"), "wb") as filep:
            filep.write(result.stdout)
        with open(os.path.join(proc_dir, "stderr.txt"), "wb") as filep:
            filep.write(result.stderr)
        with open(os.path.join(proc_dir, "exitcode.txt"), "w") as filep:
            filep.write(str(result.returncode))

        # Honor check and raise if the command failed.
        if check and result.returncode != 0:
            raise subprocess.CalledProcessError(
                result.returncode,
                result.args,
                result.stdout,
                result.stderr,
            )

        # Otherwise provide a ``TerminatedProcess`` to the caller.
        return TerminatedProcess(proc_dir=proc_dir, exitcode=result.returncode)

    def start_sudo_npte(self, argv: Argv) -> RunningProcess:
        """Start a long-running process using ``sudo npte``."""
        # Create the argument vector and the process directory.
        full_argv = self._make_argv(argv)
        print(f"+ {shlex.join(full_argv)}", file=sys.stderr)
        proc_dir = self._make_proc_dir_and_write_argv_json(full_argv)

        # Open files for collecting stdout and stderr.
        stdout_filep = open(os.path.join(proc_dir, "stdout.txt"), "wb")
        stderr_filep = open(os.path.join(proc_dir, "stderr.txt"), "wb")

        # Start the process and get its handle.
        proc = subprocess.Popen(
            full_argv,
            stdin=subprocess.DEVNULL,
            stdout=stdout_filep,
            stderr=stderr_filep,
        )

        # Provide a ``RunningProcess`` to the caller.
        return RunningProcess(
            proc_dir=proc_dir,
            proc=proc,
            files=[stdout_filep, stderr_filep],
        )

    def _make_proc_dir_and_write_argv_json(self, argv: list[str]) -> str:
        proc_dir = os.path.join(self._session_dir, str(uuid.uuid7()))
        os.makedirs(proc_dir)
        with open(os.path.join(proc_dir, "argv.json"), "w") as filep:
            json.dump(argv, filep)
        return proc_dir

    def _make_argv(self, argv: Argv) -> list[str]:
        # Start with the subcommand tokens.
        for token in argv.subcommand:
            if token.startswith("-"):
                raise ValueError(f"subcommand token {token!r} must not start with '-'")
        flat: list[str] = list(argv.subcommand)

        # Append validated flags. We always emit the `--key=value` form:
        # it is the only form binding a value to a boolean vflag flag
        # (`--sandbox true` would parse "true" as a positional).
        for name, value in sorted(argv.flags.items()):
            if not name.startswith("--"):
                raise ValueError(f"flag name {name!r} must start with '--'")
            if not value:
                raise ValueError(f"flag {name} value must not be empty")
            flat.append(f"{name}={value}")

        # Append separator and positionals.
        if argv.positionals:
            flat.append("--")
            flat.extend(argv.positionals)

        # Guard against null bytes in the complete argv.
        for arg in flat:
            if "\x00" in arg:
                raise ValueError(f"argv element {arg!r} contains null byte")

        # Prepend sudo and the npte binary path.
        return ["sudo", "-n", self._npte] + flat
