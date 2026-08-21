# SPDX-License-Identifier: GPL-3.0-or-later

import json
import os
import signal
import subprocess

import pytest

from npte.executor import Argv, NpteSudoExecutor, RunningProcess, TerminatedProcess

# --- Argv ---


def test_argv_defaults():
    a = Argv(subcommand=["lab", "create"])
    assert a.subcommand == ["lab", "create"]
    assert a.flags == {}
    assert a.positionals == []


def test_argv_is_frozen():
    a = Argv(subcommand=["lab", "create"])
    with pytest.raises(AttributeError):
        a.subcommand = ["other"]  # type: ignore[misc]


def test_argv_with_all_fields():
    a = Argv(
        subcommand=["netem", "apply"],
        flags={"--rate": "100mbit", "--delay": "25ms"},
        positionals=["router", "if-server"],
    )
    assert a.subcommand == ["netem", "apply"]
    assert a.flags == {"--rate": "100mbit", "--delay": "25ms"}
    assert a.positionals == ["router", "if-server"]


# --- _make_argv (via NpteSudoExecutor) ---


@pytest.fixture
def executor(tmp_path):
    """Build an executor whose session dir lives inside tmp_path."""
    exc = object.__new__(NpteSudoExecutor)
    exc._npte = "/usr/sbin/npte"
    exc._session_dir = str(tmp_path)
    return exc


def test_make_argv_simple(executor):
    result = executor._make_argv(Argv(subcommand=["lab", "create"]))
    assert result == ["sudo", "-n", "/usr/sbin/npte", "lab", "create"]


def test_make_argv_with_flags_sorted(executor):
    result = executor._make_argv(
        Argv(
            subcommand=["netem", "apply"],
            flags={"--rate": "100mbit", "--delay": "25ms"},
        )
    )
    assert result == [
        "sudo",
        "-n",
        "/usr/sbin/npte",
        "netem",
        "apply",
        "--delay=25ms",
        "--rate=100mbit",
    ]


def test_make_argv_with_positionals(executor):
    result = executor._make_argv(
        Argv(
            subcommand=["netns", "run"],
            flags={"--sandbox": "true"},
            positionals=["client", "./test"],
        )
    )
    assert result == [
        "sudo",
        "-n",
        "/usr/sbin/npte",
        "netns",
        "run",
        "--sandbox=true",
        "--",
        "client",
        "./test",
    ]


def test_make_argv_rejects_dash_in_subcommand(executor):
    with pytest.raises(ValueError, match="must not start with '-'"):
        executor._make_argv(Argv(subcommand=["--bad"]))


def test_make_argv_rejects_empty_flag_value(executor):
    with pytest.raises(ValueError, match="value must not be empty"):
        executor._make_argv(Argv(subcommand=["x"], flags={"--flag": ""}))


def test_make_argv_rejects_bad_flag_name(executor):
    with pytest.raises(ValueError, match="must start with '--'"):
        executor._make_argv(Argv(subcommand=["x"], flags={"flag": "value"}))


def test_make_argv_rejects_null_byte(executor):
    with pytest.raises(ValueError, match="contains null byte"):
        executor._make_argv(Argv(subcommand=["x"], positionals=["a\x00b"]))


# --- TerminatedProcess ---


def test_terminated_process_read_stdout(tmp_path):
    with open(tmp_path / "stdout.txt", "wb") as filep:
        filep.write(b"hello stdout")
    tp = TerminatedProcess(proc_dir=str(tmp_path), exitcode=0)
    assert tp.read_stdout_file() == b"hello stdout"


def test_terminated_process_read_stderr(tmp_path):
    with open(tmp_path / "stderr.txt", "wb") as filep:
        filep.write(b"hello stderr")
    tp = TerminatedProcess(proc_dir=str(tmp_path), exitcode=0)
    assert tp.read_stderr_file() == b"hello stderr"


def test_terminated_process_exitcode():
    tp = TerminatedProcess(proc_dir="/nonexistent", exitcode=42)
    assert tp.exitcode == 42
    assert tp.proc_dir == "/nonexistent"


# --- RunningProcess ---


class FakePopen:
    def __init__(self, poll_return=None, wait_return=0):
        self._poll_return = poll_return
        self._wait_return = wait_return
        self.signals_sent: list[int] = []

    def poll(self) -> int | None:
        return self._poll_return

    def wait(self, timeout: float | None = None) -> int:
        _ = timeout
        return self._wait_return

    def send_signal(self, sig: int) -> None:
        self.signals_sent.append(sig)


def test_running_process_is_running_true():
    rp = RunningProcess(proc_dir="/fake", proc=FakePopen(poll_return=None), files=[])
    assert rp.is_running() is True


def test_running_process_is_running_false():
    rp = RunningProcess(proc_dir="/fake", proc=FakePopen(poll_return=0), files=[])
    assert rp.is_running() is False


def test_running_process_wait(tmp_path):
    fake = FakePopen(wait_return=7)

    class FakeFile:
        def __init__(self):
            self.closed = False

        def close(self):
            self.closed = True

    filep1, filep2 = FakeFile(), FakeFile()
    rp = RunningProcess(proc_dir=str(tmp_path), proc=fake, files=[filep1, filep2])
    tp = rp.wait()

    assert tp.exitcode == 7
    assert tp.proc_dir == str(tmp_path)
    assert filep1.closed
    assert filep2.closed
    assert rp._files == []
    with open(tmp_path / "exitcode.txt") as filep:
        assert filep.read() == "7"


def test_running_process_kill():
    fake = FakePopen()
    rp = RunningProcess(proc_dir="/fake", proc=fake, files=[])
    rp.kill()
    assert fake.signals_sent == [signal.SIGINT]


# --- NpteSudoExecutor.__init__ ---


def test_executor_init_creates_session_dir(tmp_path, monkeypatch):
    monkeypatch.setattr("npte.executor.uuid.uuid7", lambda: "fake-uuid")
    monkeypatch.chdir(tmp_path)
    exc = NpteSudoExecutor("/usr/sbin/npte")
    expected = os.path.join(".npte", "sessions", "fake-uuid")
    assert exc._session_dir == expected
    assert os.path.isdir(expected)


def test_executor_init_custom_sessions_dir(tmp_path, monkeypatch):
    monkeypatch.setattr("npte.executor.uuid.uuid7", lambda: "fake-uuid")
    custom = str(tmp_path / "my-sessions")
    exc = NpteSudoExecutor("/usr/sbin/npte", sessions_dir=custom)
    expected = os.path.join(custom, "fake-uuid")
    assert exc._session_dir == expected
    assert os.path.isdir(expected)


# --- NpteSudoExecutor._make_proc_dir_and_write_argv_json ---


def test_make_proc_dir_and_write_argv_json(tmp_path, monkeypatch):
    monkeypatch.setattr("npte.executor.uuid.uuid7", lambda: "proc-uuid")
    exc = object.__new__(NpteSudoExecutor)
    exc._session_dir = str(tmp_path)
    proc_dir = exc._make_proc_dir_and_write_argv_json(
        ["sudo", "-n", "npte", "lab", "create"]
    )
    assert proc_dir == os.path.join(str(tmp_path), "proc-uuid")
    assert os.path.isdir(proc_dir)
    with open(os.path.join(proc_dir, "argv.json")) as filep:
        assert json.load(filep) == ["sudo", "-n", "npte", "lab", "create"]


# --- NpteSudoExecutor.run_sudo_npte ---


def test_run_sudo_npte_success(executor, monkeypatch):
    fake_result = subprocess.CompletedProcess(
        args=["sudo", "-n", "/usr/sbin/npte", "lab", "create"],
        returncode=0,
        stdout=b"out",
        stderr=b"err",
    )
    monkeypatch.setattr("npte.executor.subprocess.run", lambda *a, **kw: fake_result)
    monkeypatch.setattr("npte.executor.uuid.uuid7", lambda: "run-uuid")

    tp = executor.run_sudo_npte(Argv(subcommand=["lab", "create"]))

    assert tp.exitcode == 0
    proc_dir = tp.proc_dir
    assert os.path.basename(proc_dir) == "run-uuid"
    with open(os.path.join(proc_dir, "stdout.txt"), "rb") as filep:
        assert filep.read() == b"out"
    with open(os.path.join(proc_dir, "stderr.txt"), "rb") as filep:
        assert filep.read() == b"err"
    with open(os.path.join(proc_dir, "exitcode.txt")) as filep:
        assert filep.read() == "0"


def test_run_sudo_npte_check_raises(executor, monkeypatch):
    fake_result = subprocess.CompletedProcess(
        args=["sudo", "-n", "/usr/sbin/npte", "lab", "create"],
        returncode=1,
        stdout=b"out",
        stderr=b"err",
    )
    monkeypatch.setattr("npte.executor.subprocess.run", lambda *a, **kw: fake_result)
    monkeypatch.setattr("npte.executor.uuid.uuid7", lambda: "fail-uuid")

    with pytest.raises(subprocess.CalledProcessError) as exc_info:
        executor.run_sudo_npte(Argv(subcommand=["lab", "create"]))
    assert exc_info.value.returncode == 1


def test_run_sudo_npte_check_false_no_raise(executor, monkeypatch):
    fake_result = subprocess.CompletedProcess(
        args=["x"],
        returncode=1,
        stdout=b"",
        stderr=b"",
    )
    monkeypatch.setattr("npte.executor.subprocess.run", lambda *a, **kw: fake_result)
    monkeypatch.setattr("npte.executor.uuid.uuid7", lambda: "nocheck-uuid")

    tp = executor.run_sudo_npte(Argv(subcommand=["lab", "create"]), check=False)
    assert tp.exitcode == 1


def test_run_sudo_npte_timeout(executor, monkeypatch):
    def fake_run(*args, **kwargs):
        exc = subprocess.TimeoutExpired(cmd=args[0], timeout=5)
        exc.stdout = b"partial"
        exc.stderr = b"timeout-err"
        raise exc

    monkeypatch.setattr("npte.executor.subprocess.run", fake_run)
    monkeypatch.setattr("npte.executor.uuid.uuid7", lambda: "timeout-uuid")

    with pytest.raises(subprocess.TimeoutExpired):
        executor.run_sudo_npte(Argv(subcommand=["lab", "create"]), timeout=5)

    proc_dir = os.path.join(str(executor._session_dir), "timeout-uuid")
    with open(os.path.join(proc_dir, "stdout.txt"), "rb") as filep:
        assert filep.read() == b"partial"
    with open(os.path.join(proc_dir, "stderr.txt"), "rb") as filep:
        assert filep.read() == b"timeout-err"
    with open(os.path.join(proc_dir, "exitcode.txt")) as filep:
        assert filep.read() == "timeout"


def test_run_sudo_npte_timeout_none_output(executor, monkeypatch):
    def fake_run(*args, **kwargs):
        exc = subprocess.TimeoutExpired(cmd=args[0], timeout=5)
        exc.stdout = None
        exc.stderr = None
        raise exc

    monkeypatch.setattr("npte.executor.subprocess.run", fake_run)
    monkeypatch.setattr("npte.executor.uuid.uuid7", lambda: "tnone-uuid")

    with pytest.raises(subprocess.TimeoutExpired):
        executor.run_sudo_npte(Argv(subcommand=["lab", "create"]), timeout=5)

    proc_dir = os.path.join(str(executor._session_dir), "tnone-uuid")
    with open(os.path.join(proc_dir, "stdout.txt"), "rb") as filep:
        assert filep.read() == b""
    with open(os.path.join(proc_dir, "stderr.txt"), "rb") as filep:
        assert filep.read() == b""


# --- NpteSudoExecutor.start_sudo_npte ---


def test_start_sudo_npte(executor, monkeypatch):
    monkeypatch.setattr("npte.executor.uuid.uuid7", lambda: "start-uuid")
    monkeypatch.setattr(
        "npte.executor.subprocess.Popen",
        lambda *a, **kw: FakePopen(),
    )

    rp = executor.start_sudo_npte(
        Argv(subcommand=["netns", "run"], positionals=["server", "./srv"])
    )

    assert os.path.basename(rp.proc_dir) == "start-uuid"
    assert rp.is_running() is True
    assert len(rp._files) == 2
