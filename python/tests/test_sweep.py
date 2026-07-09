# SPDX-License-Identifier: GPL-3.0-or-later

import dataclasses
import os

import pytest

from npte import (
    NpteCell,
    NpteGrid,
    NpteLab,
    NpteShaping,
    npte_symmetric_shaping_matrix,
)
from npte.executor import Argv, RunningProcess, TerminatedProcess


# --- NpteShaping ---


def test_shaping_is_frozen():
    s = NpteShaping(rate="100mbit", delay="25ms")
    with pytest.raises(dataclasses.FrozenInstanceError):
        s.rate = "1gbit"  # type: ignore[misc]


def test_shaping_fields():
    s = NpteShaping(rate="100mbit", delay="12.5ms")
    assert s.rate == "100mbit"
    assert s.delay == "12.5ms"


# --- npte_symmetric_shaping_matrix ---


def test_symmetric_shaping_matrix_single():
    result = list(npte_symmetric_shaping_matrix(["100mbit"], [10]))
    assert result == [NpteShaping(rate="100mbit", delay="5.0ms")]


def test_symmetric_shaping_matrix_multiple():
    result = list(npte_symmetric_shaping_matrix(["10mbit", "100mbit"], [5, 25]))
    assert result == [
        NpteShaping(rate="10mbit", delay="2.5ms"),
        NpteShaping(rate="100mbit", delay="2.5ms"),
        NpteShaping(rate="10mbit", delay="12.5ms"),
        NpteShaping(rate="100mbit", delay="12.5ms"),
    ]


def test_symmetric_shaping_matrix_empty_rates():
    assert list(npte_symmetric_shaping_matrix([], [10])) == []


def test_symmetric_shaping_matrix_empty_rtts():
    assert list(npte_symmetric_shaping_matrix(["100mbit"], [])) == []


# --- NpteCell ---


def test_cell_starts_empty():
    cell = NpteCell()
    assert cell._download is None
    assert cell._upload is None
    assert cell._clients == []


def test_cell_set_shaping():
    cell = NpteCell()
    dl = NpteShaping(rate="100mbit", delay="25ms")
    ul = NpteShaping(rate="20mbit", delay="5ms")
    cell.set_download(dl)
    cell.set_upload(ul)
    assert cell._download is dl
    assert cell._upload is ul


# --- NpteGrid ---


class FakePopen:
    def __init__(self, *, running: bool = True):
        self._running = running

    def poll(self) -> int | None:
        return None if self._running else 0

    def wait(self, timeout: float | None = None) -> int:
        _ = timeout
        return 0

    def send_signal(self, sig: int) -> None:
        _ = sig
        pass


class FakeExecutor:
    """Records every call, returns canned results."""

    def __init__(self, tmp_path, *, server_running: bool = True) -> None:
        self.calls: list[tuple[str, Argv, dict]] = []
        self._server_running = server_running
        self._tmp_path = tmp_path
        self._counter = 0

    def _next_proc_dir(self) -> str:
        d = str(self._tmp_path / f"proc-{self._counter}")
        self._counter += 1
        os.makedirs(d, exist_ok=True)
        return d

    def run_sudo_npte(
        self,
        argv: Argv,
        *,
        timeout: float | None = None,
        check: bool = True,
    ) -> TerminatedProcess:
        self.calls.append(("run", argv, {"timeout": timeout, "check": check}))
        return TerminatedProcess(proc_dir=self._next_proc_dir(), exitcode=0)

    def start_sudo_npte(self, argv: Argv) -> RunningProcess:
        self.calls.append(("start", argv, {}))
        return RunningProcess(
            proc_dir=self._next_proc_dir(),
            proc=FakePopen(running=self._server_running),
            files=[],
        )


class FakeServer:
    def server_argv(self) -> list[str]:
        return ["./fake-server"]


class FakeClient:
    timeout = 30.0

    def client_argv(self, server_addr: str) -> list[str]:
        return ["./fake-client", "-c", server_addr]


def test_grid_run_one_cell(tmp_path, monkeypatch):
    monkeypatch.setattr("npte.sweep.time.sleep", lambda _: None)
    exc = FakeExecutor(tmp_path)
    lab = NpteLab(exc=exc)

    grid = NpteGrid(lab)
    grid.add_server(FakeServer())

    cell = NpteCell()
    cell.set_download(NpteShaping(rate="100mbit", delay="25ms"))
    cell.set_upload(NpteShaping(rate="20mbit", delay="5ms"))
    cell.add_client(FakeClient())
    grid.add_cell(cell)

    results = grid.run()

    assert len(results) == 1
    assert results[0].exitcode == 0

    # Call sequence: server start, netem clear+apply x2, client run
    methods = [method for method, _, _ in exc.calls]
    assert methods[0] == "start"  # server start
    assert methods[1] == "run"  # netem clear (download)
    assert methods[2] == "run"  # netem apply (download)
    assert methods[3] == "run"  # netem clear (upload)
    assert methods[4] == "run"  # netem apply (upload)
    assert methods[5] == "run"  # client run

    # Verify client ran with correct server address
    _, client_argv, client_kwargs = exc.calls[5]
    assert client_argv.positionals == [
        "client",
        "./fake-client",
        "-c",
        "172.16.2.2",
    ]
    assert client_kwargs["timeout"] == 30.0


def test_grid_run_multiple_clients_per_cell(tmp_path, monkeypatch):
    monkeypatch.setattr("npte.sweep.time.sleep", lambda _: None)
    exc = FakeExecutor(tmp_path)
    lab = NpteLab(exc=exc)

    grid = NpteGrid(lab)
    grid.add_server(FakeServer())

    cell = NpteCell()
    cell.set_download(NpteShaping(rate="100mbit", delay="25ms"))
    cell.set_upload(NpteShaping(rate="100mbit", delay="25ms"))
    cell.add_client(FakeClient())
    cell.add_client(FakeClient())
    grid.add_cell(cell)

    results = grid.run()
    assert len(results) == 2


def test_grid_run_server_exits_early(tmp_path, monkeypatch):
    monkeypatch.setattr("npte.sweep.time.sleep", lambda _: None)
    exc = FakeExecutor(tmp_path, server_running=False)
    lab = NpteLab(exc=exc)

    grid = NpteGrid(lab)
    grid.add_server(FakeServer())

    cell = NpteCell()
    cell.add_client(FakeClient())
    grid.add_cell(cell)

    with pytest.raises(RuntimeError, match="server exited early"):
        grid.run()


def test_grid_run_no_shaping(tmp_path, monkeypatch):
    monkeypatch.setattr("npte.sweep.time.sleep", lambda _: None)
    exc = FakeExecutor(tmp_path)
    lab = NpteLab(exc=exc)

    grid = NpteGrid(lab)
    grid.add_server(FakeServer())

    cell = NpteCell()
    cell.add_client(FakeClient())
    grid.add_cell(cell)

    results = grid.run()
    assert len(results) == 1

    # With no shaping set, _apply_shaping calls shape_download()/shape_upload()
    # with no args, which just clears (no apply)
    netem_calls = [
        (method, argv)
        for method, argv, _ in exc.calls
        if argv.subcommand == ["netem", "clear"]
    ]
    assert len(netem_calls) == 2
