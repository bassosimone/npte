"""Internal module implementing ``NpteLab`` and related types."""

# SPDX-License-Identifier: GPL-3.0-or-later

from typing import Protocol

from .executor import Argv, NpteSudoExecutor, RunningProcess, TerminatedProcess


class ExecutorProtocol(Protocol):
    """Structural protocol for executing ``npte`` subcommands."""

    def run_sudo_npte(
        self,
        argv: Argv,
        *,
        timeout: float | None = None,
        check: bool = True,
    ) -> TerminatedProcess: ...

    def start_sudo_npte(self, argv: Argv) -> RunningProcess: ...


class Namespace:
    """
    Model a generic ``npte`` namespace.

    Processes run through this class execute inside the namespace with
    the ``npte`` sandbox enabled (``--sandbox=true``).
    """

    def __init__(self, *, exc: ExecutorProtocol, addr: str, name: str) -> None:
        self.addr = addr
        self.name = name
        self._exc = exc

    def start(self, argv: list[str]) -> RunningProcess:
        """
        Start a long-running process inside this network namespace.

        This is how you typically run a server process.
        """
        return self._exc.start_sudo_npte(self._make_argv(argv))

    def run(
        self,
        argv: list[str],
        *,
        timeout: float | None = None,
        check: bool = True,
    ) -> TerminatedProcess:
        """
        Run a short-lived process inside this network namespace.

        This is how you typically run a client process.

        See ``NpteSudoExecutor.run_sudo_npte`` for the ``timeout`` caveat.
        """
        return self._exc.run_sudo_npte(
            self._make_argv(argv),
            timeout=timeout,
            check=check,
        )

    def _make_argv(self, argv: list[str]) -> Argv:
        return Argv(
            subcommand=["netns", "run"],
            flags={"--sandbox": "true"},
            positionals=[self.name] + argv,
        )


class ClientNamespace(Namespace):
    """Model the ``npte`` namespace simulating the client."""

    def shape_download(
        self,
        *,
        rate: str | None = None,
        delay: str | None = None,
    ) -> None:
        """
        Shape the client download path (router→client traffic).

        Because netem shapes egress only, we act on the router's
        ``if-client`` interface, whose egress faces the client.

        The rate and delay values use tc syntax (e.g., ``100mbit``,
        ``25ms``). When both are None we just clear the previous
        shaping configuration without applying changes.
        """
        self._netem_apply(ifname="if-client", rate=rate, delay=delay)

    def shape_upload(
        self,
        *,
        rate: str | None = None,
        delay: str | None = None,
    ) -> None:
        """
        Shape the client upload path (client→server traffic).

        Because netem shapes egress only, we act on the router's
        ``if-server`` interface, whose egress faces the server.

        The rate and delay values use tc syntax (e.g., ``100mbit``,
        ``25ms``). When both are None we just clear the previous
        shaping configuration without applying changes.
        """
        self._netem_apply(ifname="if-server", rate=rate, delay=delay)

    def _netem_apply(
        self,
        *,
        ifname: str,
        rate: str | None,
        delay: str | None,
    ) -> None:
        # Clear the previous configuration as a side effect.
        self._exc.run_sudo_npte(
            Argv(
                subcommand=["netem", "clear"],
                positionals=["router", ifname],
            )
        )

        # Do nothing without a configuration to apply.
        if rate is None and delay is None:
            return

        # Build the flags dict from the provided values.
        flags: dict[str, str] = {}
        if rate is not None:
            flags["--rate"] = rate
        if delay is not None:
            flags["--delay"] = delay

        # Apply the shaping configuration.
        self._exc.run_sudo_npte(
            Argv(
                subcommand=["netem", "apply"],
                flags=flags,
                positionals=["router", ifname],
            )
        )


class NpteLab:
    """
    Model the ``npte`` based laboratory in Python.

    This class implements the context manager protocol so that the lab is
    created when you enter and destroyed when you exit.

    The ``client`` (172.16.3.2) and ``server`` (172.16.2.2) attributes
    model the leaf namespaces of the client-router-server topology.
    """

    def __init__(
        self,
        *,
        npte: str = "/usr/sbin/npte",
        exc: ExecutorProtocol | None = None,
        sessions_dir: str | None = None,
    ) -> None:
        exc = (
            NpteSudoExecutor(
                npte,
                sessions_dir=sessions_dir,
            )
            if exc is None
            else exc
        )
        self.client = ClientNamespace(exc=exc, addr="172.16.3.2", name="client")
        self.server = Namespace(exc=exc, addr="172.16.2.2", name="server")
        self._exc = exc

    def __enter__(self):
        self._exc.run_sudo_npte(Argv(subcommand=["lab", "create"]))
        return self

    def __exit__(self, exc_type, exc_value, traceback):
        _, _, _ = exc_type, exc_value, traceback
        self._exc.run_sudo_npte(Argv(subcommand=["lab", "destroy"]))
