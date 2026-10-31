#!/usr/bin/env python3
# SPDX-License-Identifier: MIT
"""Self-test of one adapter image: the contract, then client against device.

    scripts/selftest.py <adapter>

VERSION and REGISTRY name the image (the Makefile exports both). REPORT_DIR
keeps every document; results.json goes there, or to the current directory.
Runs on a private Docker network and publishes nothing.
"""

import json
import os
import subprocess
import sys
import time
import uuid
from pathlib import Path

import interop_result as ir

ROOT = Path(__file__).resolve().parent.parent
LABEL = "sunspec-interop.run"
FIXTURES = ("baseline", "edge", "altbase")
WRITE_MODEL, WRITE_POINT, WRITE_VALUE = 123, "WMaxLimPct", 750
READONLY_MODEL, READONLY_POINT = 103, "W"
FLOOR = 8


class Run:
    def __init__(self, adapter):
        self.adapter = adapter
        self.image = f"{os.environ.get('REGISTRY', 'ghcr.io/otfabric')}/sunspec-interop-{adapter}:{os.environ.get('VERSION', 'dev')}"
        self.network = f"sunspec-interop-{uuid.uuid4().hex[:8]}"
        self.containers = []
        self.results = []
        self.reports = Path(os.environ.get("REPORT_DIR", "."))
        self.reports.mkdir(parents=True, exist_ok=True)
        self.count = 0

    def docker(self, *args, timeout=60):
        return subprocess.run(["docker", *args], capture_output=True, text=True, timeout=timeout)

    def run(self, *args):
        return self.docker("run", "--rm", "--network", self.network, "--label", LABEL, self.image, *args)

    def keep(self, stdout):
        self.count += 1
        (self.reports / f"{self.adapter}-{self.count:03d}.json").write_text(stdout)

    def case(self, scenario, fn):
        started = time.monotonic()
        try:
            reason = fn()
            outcome = "pass" if reason is None else "fail"
        except (subprocess.TimeoutExpired, OSError, ValueError, KeyError, RuntimeError) as e:
            outcome, reason = "infra-error", repr(e)
        r = {"id": scenario, "suite": scenario.split(".", 1)[0], "roles": {"client": self.adapter, "server": self.adapter},
             "outcome": outcome, "durationMs": int((time.monotonic() - started) * 1000)}
        if reason is not None:
            r["reason"] = reason
        self.results.append(r)
        print(f"[selftest] {outcome:11} {scenario} {reason or ''}", flush=True)

    def server(self, fixture):
        name = f"{self.network}-{fixture}-{len(self.containers)}"
        p = self.docker("run", "-d", "--rm", "--network", self.network, "--network-alias", name, "--name", name, "--label", LABEL, self.image,
                        "server", "--fixture", f"/fixtures/{fixture}/fixture.json")
        if p.returncode != 0:
            raise RuntimeError(p.stderr)
        self.containers.append(name)
        for _ in range(100):
            if self.docker("exec", name, "test", "-f", "/run/sunspec-interop/ready").returncode == 0:
                return name
            time.sleep(0.1)
        raise RuntimeError(f"{name} never became ready: " + self.docker("logs", name).stderr)

    def client(self, *args):
        p = self.run("client", *args)
        doc = json.loads(p.stdout) if p.stdout.strip() else None
        if doc is not None:
            self.keep(p.stdout)
        return p.returncode, doc, p.stderr

    def close(self):
        for c in self.containers:
            self.docker("rm", "-f", c)
        self.docker("network", "rm", self.network)


def compare(want, got, where=""):
    """The first difference between what a fixture sets and what was read."""
    if isinstance(want, list):
        if not isinstance(got, list) or len(want) != len(got):
            return f"{where}: {len(want)} instances, read {got!r}"
        for i, (w, g) in enumerate(zip(want, got)):
            if (d := compare(w, g, f"{where}[{i}]")) is not None:
                return d
        return None
    if isinstance(want, dict):
        for k, w in want.items():
            if (d := compare(w, got.get(k), f"{where}.{k}")) is not None:
                return d
        return None
    if isinstance(want, float) and isinstance(got, (int, float)):
        return None if abs(want - got) <= abs(want) * 1e-6 else f"{where}: want {want}, read {got}"
    return None if want == got else f"{where}: want {want!r}, read {got!r}"


def contract(run):
    def capabilities():
        p = run.run("print-capabilities")
        run.keep(p.stdout)
        doc = json.loads(p.stdout)
        if doc["adapter"] != run.adapter or doc["upstream"]["version"] in ("", "unknown"):
            return f"capabilities: {doc}"
        return None if sorted(doc["fixtures"]) == sorted(FIXTURES) else f"fixtures: {doc['fixtures']}"

    def registers():
        for f in FIXTURES:
            doc = json.loads(run.run("print-registers", f).stdout)
            r = doc["registers"]
            if r[:2] != [0x5375, 0x6E53] or r[-2:] != [0xFFFF, 0]:
                return f"{f}: no marker or end model"
        return None

    def usage():
        codes = (run.run("no-such-command").returncode, run.run("client", "scan").returncode,
                 run.run("client", "scan", "--host", "x", "--nope").returncode)
        return None if codes == (2, 2, 2) else f"exit codes {codes}"

    def connect():
        code, doc, _ = run.client("scan", "--host", "no-such-host.invalid", "--timeout", "1")
        return None if code == 3 and doc and not doc["ok"] else f"exit {code}: {doc}"

    def bad_fixture():
        p = run.run("server", "--fixture", "/nonexistent/fixture.json")
        return None if p.returncode == 4 else f"exit {p.returncode}"

    run.case("contract.capabilities", capabilities)
    run.case("contract.registers", registers)
    run.case("contract.usage-error", usage)
    run.case("contract.connect-error", connect)
    run.case("contract.fixture-error", bad_fixture)


def scans(run):
    for name in FIXTURES:
        fx = json.loads((ROOT / "fixtures" / name / "fixture.json").read_text())

        def scan(fx=fx, name=name):
            host = run.server(name)
            code, doc, err = run.client("scan", "--host", host, "--unit-id", str(fx["unitId"]))
            if code != 0:
                return f"exit {code}: {doc} {err}"
            res = doc["result"]
            if res["baseAddress"] != fx["baseAddress"]:
                return f"base address {res['baseAddress']}"
            if [m["id"] for m in res["models"]] != [m["ID"] for m in fx["models"]]:
                return f"models {[m['id'] for m in res['models']]}"
            for want, got in zip(fx["models"], res["models"]):
                if "error" in got:
                    return f"model {got['id']}: {got['error']}"
                if (d := compare(want, got["values"], f"model {got['id']}")) is not None:
                    return d
            return None

        run.case(f"device.scan.{name}", scan)


def writes(run):
    fx = json.loads((ROOT / "fixtures" / "baseline" / "fixture.json").read_text())

    def write():
        host = run.server("baseline")
        code, doc, err = run.client("write", "--host", host, "--model", str(WRITE_MODEL), "--point", WRITE_POINT, "--value", str(WRITE_VALUE))
        if code != 0 or doc["result"]["value"] != WRITE_VALUE:
            return f"exit {code}: {doc} {err}"
        _, doc, _ = run.client("scan", "--host", host)
        got = next(m for m in doc["result"]["models"] if m["id"] == WRITE_MODEL)["values"][WRITE_POINT]
        return None if got == WRITE_VALUE else f"read back {got}"

    def readonly():
        host = run.server("baseline")
        code, doc, _ = run.client("write", "--host", host, "--model", str(READONLY_MODEL), "--point", READONLY_POINT, "--value", "1")
        if code != 1 or doc["ok"]:
            return f"a read-only point was written: exit {code}: {doc}"
        _, doc, _ = run.client("scan", "--host", host)
        want = next(m for m in fx["models"] if m["ID"] == READONLY_MODEL)[READONLY_POINT]
        got = next(m for m in doc["result"]["models"] if m["id"] == READONLY_MODEL)["values"][READONLY_POINT]
        return None if got == want else f"value changed to {got}"

    def wrong_unit():
        host = run.server("baseline")
        code, doc, _ = run.client("scan", "--host", host, "--unit-id", "9")
        return None if code != 0 and doc and not doc["ok"] else f"exit {code}: {doc}"

    run.case("device.write.read-write-point", write)
    run.case("device.write.read-only-point", readonly)
    run.case("device.unit-id.other-unit", wrong_unit)


def main(argv):
    if len(argv) != 1:
        print(__doc__, file=sys.stderr)
        return 2
    run = Run(argv[0])
    started = ir.now()
    upstream = json.loads(subprocess.run(["docker", "run", "--rm", run.image, "print-capabilities"], capture_output=True, text=True, check=True).stdout)
    run.docker("network", "create", run.network)
    try:
        contract(run)
        scans(run)
        writes(run)
    finally:
        run.close()
    implementation = {"name": run.adapter, "version": upstream["upstream"]["version"]}
    if digest := ir.repo_digest(run.image):
        implementation["image"] = digest
    doc = {
        "schemaVersion": "1.0", "repository": "otfabric/sunspec-interop", "gitSha": ir.git_sha(str(ROOT)), "startedAt": started, "finishedAt": ir.now(),
        "host": ir.host(), "implementations": [implementation], "reproduce": f"make image-{run.adapter} interop", "results": run.results,
    }
    path = run.reports / "results.json"
    ir.write(str(path), doc)
    bad = ["schema: " + e for e in ir.schema_errors(doc)] + ir.problems(doc, FLOOR)
    for b in bad:
        print("FAIL " + b, file=sys.stderr)
    print(f"[interop] {doc['summary']['pass']} passed, {len(doc['results']) - doc['summary']['pass']} other")
    return 1 if bad else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
