"""
Drive ``npte`` from Python scripts.

The main assumption is that you have used ``npte sudoers`` to install
sudoers(5) rules that allow executing the following commands:

1. ``npte netns *``
2. ``npte netem *``
3. ``npte lab *``

with sudo(8) without requiring a password prompt.

Use the ``NpteLab`` type to create a laboratory and manage its lifecycle:

    from npte import NpteLab

    with NpteLab() as lab:
        server = lab.server.start(["./httpserver"])
        lab.client.shape_download(rate="100mbit", delay="25ms")
        lab.client.shape_upload(rate="20mbit", delay="25ms")
        lab.client.run(["curl", f"http://{lab.server.addr}:8080/"])
        server.kill()
        server.wait()
"""

# SPDX-License-Identifier: GPL-3.0-or-later

from .executor import NpteSudoExecutor
from .lab import NpteLab

__all__ = ["NpteLab", "NpteSudoExecutor"]
