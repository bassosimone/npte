# SPDX-License-Identifier: GPL-3.0-or-later

import pytest

from npte import NpteIperf3Client, NpteIperf3Server


# --- NpteIperf3Server ---


def test_server_defaults():
    s = NpteIperf3Server()
    assert s.binary == "iperf3"
    assert s.port == 5201


def test_server_is_frozen():
    s = NpteIperf3Server()
    with pytest.raises(AttributeError):
        s.binary = "other"  # type: ignore[misc]


def test_server_argv():
    s = NpteIperf3Server()
    assert s.server_argv() == ["iperf3", "-s", "-J", "-p", "5201"]


def test_server_argv_custom_port():
    s = NpteIperf3Server(port=9999)
    argv = s.server_argv()
    assert "-p" in argv
    assert argv[argv.index("-p") + 1] == "9999"


# --- NpteIperf3Client ---


def test_client_defaults():
    c = NpteIperf3Client()
    assert c.binary == "iperf3"
    assert c.cc == "cubic"
    assert c.duration == "10"
    assert c.port == 5201
    assert c.timeout == 30


def test_client_is_frozen():
    c = NpteIperf3Client()
    with pytest.raises(AttributeError):
        c.cc = "bbr"  # type: ignore[misc]


def test_client_argv():
    c = NpteIperf3Client(cc="bbr")
    assert c.client_argv("172.16.2.2") == [
        "/usr/bin/time",
        "-p",
        "iperf3",
        "-c",
        "172.16.2.2",
        "-p",
        "5201",
        "-R",
        "-J",
        "--get-server-output",
        "-t",
        "10",
        "-C",
        "bbr",
    ]


def test_client_argv_custom_duration():
    c = NpteIperf3Client(cc="cubic", duration="30")
    assert c.client_argv("10.0.0.1") == [
        "/usr/bin/time",
        "-p",
        "iperf3",
        "-c",
        "10.0.0.1",
        "-p",
        "5201",
        "-R",
        "-J",
        "--get-server-output",
        "-t",
        "30",
        "-C",
        "cubic",
    ]
