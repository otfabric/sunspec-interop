#!/usr/bin/env python3
# SPDX-License-Identifier: MIT
"""Compares the upstream pin of one adapter with the latest upstream release.

    scripts/latest-upstream.py <adapter>

Prints key=value lines (the format of $GITHUB_OUTPUT):

    pin=<version in the adapter Dockerfile>
    latest=<latest stable upstream release>
    latest_sha=<its commit, for an adapter that is pinned by commit>
    update=yes|no
    source=<where the latest release was looked up>

Nothing is modified. Pre-releases are never "latest".
"""

import json
import re
import sys
import urllib.request
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
AGENT = "sunspec-interop latest-upstream (https://github.com/otfabric/sunspec-interop)"


def fetch(url):
    request = urllib.request.Request(url, headers={"User-Agent": AGENT, "Accept": "application/json, */*"})
    with urllib.request.urlopen(request, timeout=30) as response:
        return response.read().decode()


def key(version):
    """Sort key of a stable version; None for anything else (pre-releases)."""
    match = re.fullmatch(r"v?(\d+(?:\.\d+)*)", version)
    return tuple(int(part) for part in match.group(1).split(".")) if match else None


def newest(versions):
    stable = [v for v in versions if key(v) is not None]
    if not stable:
        raise SystemExit("no stable release found upstream")
    return max(stable, key=key)


def pysunspec2():
    releases = json.loads(fetch("https://pypi.org/pypi/pysunspec2/json"))["releases"]
    live = [v for v, files in releases.items() if files and not all(f.get("yanked") for f in files)]
    return newest(live), "", "PyPI"


def cbsigma():
    # cB-Sigma publishes git tags, no GitHub releases. The device module,
    # sunspec-sim, is tagged with the library and stays at its pin in a
    # candidate build (adapters/cbsigma/Dockerfile, SIM_VERSION).
    tags = json.loads(fetch("https://api.github.com/repos/cB-Sigma/sunspec/tags?per_page=100"))
    latest = newest([t["name"] for t in tags])
    sha = next(t["commit"]["sha"] for t in tags if t["name"] == latest)
    return latest, sha, "GitHub tags of cB-Sigma/sunspec"


RESOLVERS = {"pysunspec2": pysunspec2, "cbsigma": cbsigma}


def main():
    if len(sys.argv) != 2 or sys.argv[1] not in RESOLVERS:
        raise SystemExit(f"usage: latest-upstream.py {'|'.join(RESOLVERS)}")
    adapter = sys.argv[1]
    dockerfile = (ROOT / "adapters" / adapter / "Dockerfile").read_text()
    match = re.search(r"^ARG UPSTREAM_VERSION=(.+)$", dockerfile, re.M)
    if not match:
        raise SystemExit(f"adapters/{adapter}/Dockerfile has no ARG UPSTREAM_VERSION=<pin>")
    pin = match.group(1).strip()
    latest, latest_sha, source = RESOLVERS[adapter]()
    print(f"pin={pin}")
    print(f"latest={latest}")
    print(f"latest_sha={latest_sha}")
    print(f"update={'yes' if key(latest) != key(pin) else 'no'}")
    print(f"source={source}")


if __name__ == "__main__":
    main()
