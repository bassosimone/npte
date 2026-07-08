# SPDX-License-Identifier: GPL-3.0-or-later

from npte.executor import Argv, RunningProcess, TerminatedProcess
from npte.lab import ClientNamespace, Namespace, NpteLab


class FakeExecutor:
    """Records every call for later assertion."""

    def __init__(self) -> None:
        self.calls: list[tuple[str, Argv, dict]] = []

    def run_sudo_npte(
        self,
        argv: Argv,
        *,
        timeout: float | None = None,
        check: bool = True,
    ) -> TerminatedProcess:
        self.calls.append(("run", argv, {"timeout": timeout, "check": check}))
        return TerminatedProcess(proc_dir="/fake", exitcode=0)

    def start_sudo_npte(self, argv: Argv) -> RunningProcess:
        self.calls.append(("start", argv, {}))
        return RunningProcess(proc_dir="/fake", proc=FakePopen(), files=[])


class FakePopen:
    def poll(self) -> int | None:
        return 0

    def wait(self, timeout: float | None = None) -> int:
        return 0

    def send_signal(self, sig: int) -> None:
        pass


# --- Namespace ---


def test_namespace_run_builds_correct_argv():
    exc = FakeExecutor()
    ns = Namespace(exc=exc, addr="10.0.0.1", name="myns")
    ns.run(["./server", "--port", "8080"])

    assert len(exc.calls) == 1
    method, argv, kwargs = exc.calls[0]
    assert method == "run"
    assert argv.subcommand == ["netns", "run"]
    assert argv.flags == {"--sandbox": "true"}
    assert argv.positionals == ["myns", "./server", "--port", "8080"]
    assert kwargs == {"timeout": None, "check": True}


def test_namespace_run_passes_timeout_and_check():
    exc = FakeExecutor()
    ns = Namespace(exc=exc, addr="10.0.0.1", name="myns")
    ns.run(["./client"], timeout=30.0, check=False)

    _, _, kwargs = exc.calls[0]
    assert kwargs == {"timeout": 30.0, "check": False}


def test_namespace_start_builds_correct_argv():
    exc = FakeExecutor()
    ns = Namespace(exc=exc, addr="10.0.0.1", name="myns")
    ns.start(["./server", "--port", "8080"])

    assert len(exc.calls) == 1
    method, argv, _ = exc.calls[0]
    assert method == "start"
    assert argv.subcommand == ["netns", "run"]
    assert argv.flags == {"--sandbox": "true"}
    assert argv.positionals == ["myns", "./server", "--port", "8080"]


# --- ClientNamespace ---


def test_shape_download_clear_only():
    exc = FakeExecutor()
    ns = ClientNamespace(exc=exc, addr="10.0.0.1", name="client")
    ns.shape_download()

    assert len(exc.calls) == 1
    _, argv, _ = exc.calls[0]
    assert argv.subcommand == ["netem", "clear"]
    assert argv.positionals == ["router", "if-client"]


def test_shape_download_with_rate_and_delay():
    exc = FakeExecutor()
    ns = ClientNamespace(exc=exc, addr="10.0.0.1", name="client")
    ns.shape_download(rate="100mbit", delay="25ms")

    assert len(exc.calls) == 2
    _, clear_argv, _ = exc.calls[0]
    assert clear_argv.subcommand == ["netem", "clear"]
    assert clear_argv.positionals == ["router", "if-client"]

    _, apply_argv, _ = exc.calls[1]
    assert apply_argv.subcommand == ["netem", "apply"]
    assert apply_argv.flags == {"--rate": "100mbit", "--delay": "25ms"}
    assert apply_argv.positionals == ["router", "if-client"]


def test_shape_download_with_rate_only():
    exc = FakeExecutor()
    ns = ClientNamespace(exc=exc, addr="10.0.0.1", name="client")
    ns.shape_download(rate="1gbit")

    assert len(exc.calls) == 2
    _, apply_argv, _ = exc.calls[1]
    assert apply_argv.flags == {"--rate": "1gbit"}


def test_shape_download_with_delay_only():
    exc = FakeExecutor()
    ns = ClientNamespace(exc=exc, addr="10.0.0.1", name="client")
    ns.shape_download(delay="10ms")

    assert len(exc.calls) == 2
    _, apply_argv, _ = exc.calls[1]
    assert apply_argv.flags == {"--delay": "10ms"}


def test_shape_upload_clear_only():
    exc = FakeExecutor()
    ns = ClientNamespace(exc=exc, addr="10.0.0.1", name="client")
    ns.shape_upload()

    assert len(exc.calls) == 1
    _, argv, _ = exc.calls[0]
    assert argv.subcommand == ["netem", "clear"]
    assert argv.positionals == ["router", "if-server"]


def test_shape_upload_with_rate_and_delay():
    exc = FakeExecutor()
    ns = ClientNamespace(exc=exc, addr="10.0.0.1", name="client")
    ns.shape_upload(rate="100mbit", delay="25ms")

    assert len(exc.calls) == 2
    _, apply_argv, _ = exc.calls[1]
    assert apply_argv.subcommand == ["netem", "apply"]
    assert apply_argv.flags == {"--rate": "100mbit", "--delay": "25ms"}
    assert apply_argv.positionals == ["router", "if-server"]


# --- NpteLab ---


def test_lab_enter_calls_lab_create():
    exc = FakeExecutor()
    lab = NpteLab(exc=exc)
    lab.__enter__()

    assert len(exc.calls) == 1
    _, argv, _ = exc.calls[0]
    assert argv.subcommand == ["lab", "create"]


def test_lab_exit_calls_lab_destroy():
    exc = FakeExecutor()
    lab = NpteLab(exc=exc)
    lab.__enter__()
    exc.calls.clear()
    lab.__exit__(None, None, None)

    assert len(exc.calls) == 1
    _, argv, _ = exc.calls[0]
    assert argv.subcommand == ["lab", "destroy"]


def test_lab_context_manager():
    exc = FakeExecutor()
    with NpteLab(exc=exc) as lab:
        assert lab.client.addr == "172.16.3.2"
        assert lab.client.name == "client"
        assert lab.server.addr == "172.16.2.2"
        assert lab.server.name == "server"

    assert len(exc.calls) == 2
    assert exc.calls[0][1].subcommand == ["lab", "create"]
    assert exc.calls[1][1].subcommand == ["lab", "destroy"]
