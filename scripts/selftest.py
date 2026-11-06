#!/usr/bin/env python3
# SPDX-License-Identifier: MIT
"""The self-tests behind scripts/smoke.sh and scripts/interop.sh.

    scripts/selftest.py smoke <adapter> <image>
    scripts/selftest.py interop <adapter>=<image> [<adapter>=<image> ...]

smoke checks the contract of one image on its own. interop runs every case of
tests/cases.tsv for every client against every server, the same adapter on
both sides included, and writes results.json (interop-result-v1). The shell
scripts name the images (scripts/lib.sh, image_of); this file knows no image
of its own. REPORT_DIR keeps every document; results.json goes there, or to
the current directory. Runs on a private Docker network and publishes
nothing.
"""

import json
import os
import shlex
import subprocess
import sys
import time
import uuid
from pathlib import Path

import interop_result as ir

ROOT = Path(__file__).resolve().parent.parent
LABEL = "sunspec-interop.run"
FIXTURES = ("baseline", "edge", "altbase")
CASES = ROOT / "tests" / "cases.tsv"
FLOOR = 6


def docker(*args, timeout=60):
    return subprocess.run(["docker", *args], capture_output=True, text=True, timeout=timeout)


class Run:
    """One Docker network, the containers on it, and what the tests found."""

    def __init__(self, images):
        self.images = images
        self.network = f"sunspec-interop-{uuid.uuid4().hex[:8]}"
        self.containers = []
        self.results = []
        self.failed = 0
        self.reports = Path(os.environ.get("REPORT_DIR", "."))
        self.reports.mkdir(parents=True, exist_ok=True)
        self.count = 0
        self.caps = {}
        for name, image in images.items():
            p = docker("run", "--rm", image, "print-capabilities")
            if p.returncode != 0:
                raise SystemExit(f"{name}: print-capabilities of {image} failed: {p.stderr}")
            self.caps[name] = json.loads(p.stdout)
        docker("network", "create", self.network)

    def run(self, adapter, *args):
        return docker("run", "--rm", "--network", self.network, "--label", LABEL, self.images[adapter], *args)

    def keep(self, label, stdout):
        if os.environ.get("REPORT_DIR"):
            self.count += 1
            (self.reports / f"{label}-{self.count:03d}.json").write_text(stdout)

    def has_role(self, adapter, role):
        return role in self.caps[adapter].get("roles", [])

    def has_operation(self, adapter, operation):
        return operation in self.caps[adapter].get("clientOperations", [])

    def case(self, scenario, roles, fn):
        started = time.monotonic()
        try:
            reason = fn()
            outcome = "pass" if reason is None else "fail"
        except (subprocess.TimeoutExpired, OSError, ValueError, KeyError, RuntimeError, StopIteration) as e:
            outcome, reason = "infra-error", repr(e)
        self.record(scenario, roles, outcome, reason, started)

    def record(self, scenario, roles, outcome, reason, started=None):
        r = {"id": scenario, "suite": scenario.split(".", 1)[0], "roles": roles, "outcome": outcome,
             "durationMs": int((time.monotonic() - (started or time.monotonic())) * 1000)}
        if reason is not None:
            r["reason"] = reason
        self.results.append(r)
        if outcome in ("fail", "infra-error"):
            self.failed += 1
        pair = "->".join(roles[k] for k in ("client", "server") if k in roles)
        print(f"[{sys.argv[1]}] {outcome:11} {scenario} {pair} {reason or ''}", flush=True)

    def server(self, adapter, fixture):
        name = f"{self.network}-{adapter}-{fixture}-{len(self.containers)}"
        p = docker("run", "-d", "--rm", "--network", self.network, "--network-alias", name, "--name", name, "--label", LABEL,
                   self.images[adapter], "server", "--fixture", f"/fixtures/{fixture}/fixture.json")
        if p.returncode != 0:
            raise RuntimeError(p.stderr)
        self.containers.append(name)
        for _ in range(300):
            if docker("exec", name, "test", "-f", "/run/sunspec-interop/ready").returncode == 0:
                return name
            time.sleep(0.1)
        raise RuntimeError(f"{name} never became ready: " + docker("logs", name).stderr)

    def client(self, adapter, *args):
        p = self.run(adapter, "client", *args)
        doc = json.loads(p.stdout) if p.stdout.strip() else None
        if doc is not None:
            self.keep(adapter, p.stdout)
        return p.returncode, doc, p.stderr

    def close(self):
        for c in self.containers:
            docker("rm", "-f", c)
        docker("network", "rm", self.network)


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
        if not isinstance(got, dict):
            return f"{where}: read {got!r}"
        for k, w in want.items():
            if (d := compare(w, got.get(k), f"{where}.{k}")) is not None:
                return d
        return None
    if isinstance(want, float) and isinstance(got, (int, float)):
        return None if abs(want - got) <= abs(want) * 1e-6 else f"{where}: want {want}, read {got}"
    return None if want == got else f"{where}: want {want!r}, read {got!r}"


def fixture(name):
    return json.loads((ROOT / "fixtures" / name / "fixture.json").read_text())


def point(doc, model, name):
    return next(m for m in doc["result"]["models"] if m["id"] == model)["values"][name]


def flag(args, name):
    return int(args[args.index(name) + 1], 0)


# ---- smoke: the contract of one image ----

def smoke(adapter, image):
    run = Run({adapter: image})
    roles = {"harness": "smoke", "server": adapter}
    caps = run.caps[adapter]
    if os.environ.get("REPORT_DIR"):
        (run.reports / f"capabilities-{adapter}.json").write_text(json.dumps(caps, indent=1, sort_keys=True) + "\n")

    def capabilities():
        if caps["adapter"] != adapter or caps["upstream"]["version"] in ("", "unknown"):
            return f"capabilities: {caps}"
        engine = caps["upstream"].get("independentEngine", True)
        if not isinstance(engine, bool):
            return f"upstream.independentEngine is {engine!r}"
        return None if sorted(caps["fixtures"]) == sorted(FIXTURES) else f"fixtures: {caps['fixtures']}"

    def fixtures():
        for f in FIXTURES:
            if json.loads(run.run(adapter, "print-fixture", f).stdout) != fixture(f):
                return f"print-fixture {f} differs from fixtures/{f}/fixture.json"
        return None

    def registers():
        for f in FIXTURES:
            doc = json.loads(run.run(adapter, "print-registers", f).stdout)
            r = doc["registers"]
            if r[:2] != [0x5375, 0x6E53] or r[-2:] != [0xFFFF, 0]:
                return f"{f}: no marker or end model"
            if doc["baseAddress"] != fixture(f)["baseAddress"] or doc["unitId"] != fixture(f)["unitId"]:
                return f"{f}: base address {doc['baseAddress']}, unit {doc['unitId']}"
        return None

    def usage():
        codes = (run.run(adapter, "no-such-command").returncode, run.run(adapter, "client", "scan").returncode,
                 run.run(adapter, "client", "scan", "--host", "x", "--nope").returncode)
        return None if codes == (2, 2, 2) else f"exit codes {codes}"

    def unsupported():
        # An operation the adapter does not declare is a usage error, never a fake success.
        for op in ("scan", "write"):
            if run.has_operation(adapter, op):
                continue
            code = run.run(adapter, "client", op, "--host", "x", "--model", "1", "--point", "Mn", "--value", "1").returncode
            if code != 2:
                return f"undeclared {op}: exit {code}"
        return None

    def connect():
        code, doc, _ = run.client(adapter, "scan", "--host", "no-such-host.invalid", "--timeout", "1")
        return None if code == 3 and doc and not doc["ok"] and doc["error"]["kind"] == "connect" else f"exit {code}: {doc}"

    def bad_fixture():
        p = run.run(adapter, "server", "--fixture", "/nonexistent/fixture.json")
        return None if p.returncode == 4 else f"exit {p.returncode}"

    def ready():
        name = run.server(adapter, "baseline")
        user = docker("exec", name, "id", "-u").stdout.strip()
        health = docker("inspect", "--format", "{{if .State.Health}}yes{{end}}", name).stdout.strip()
        docker("stop", "-t", "10", name)
        return None if user == "10001" and health == "yes" else f"uid {user}, health check {health or 'none'}"

    try:
        run.case("contract.capabilities", roles, capabilities)
        run.case("contract.fixtures", roles, fixtures)
        run.case("contract.registers", roles, registers)
        run.case("contract.usage-error", roles, usage)
        run.case("contract.unsupported-operation", roles, unsupported)
        run.case("contract.fixture-error", roles, bad_fixture)
        run.case("contract.ready", roles, ready)
        if run.has_role(adapter, "client"):
            run.case("contract.connect-error", roles, connect)
    finally:
        run.close()
    passed = sum(r["outcome"] == "pass" for r in run.results)
    print(f"[smoke] {adapter}: {passed} passed, {run.failed} failed")
    return 1 if run.failed or not passed else 0


# ---- interop: every client against every server ----

def cases():
    out = []
    for line in CASES.read_text().splitlines():
        if not line.strip() or line.startswith("#"):
            continue
        cid, fx, operation, args, check = line.split("\t")
        out.append((cid, fx, operation, [] if args == "-" else shlex.split(args), check))
    return out


def device_case(run, client, server, fx_name, operation, args, check):
    fx = fixture(fx_name)
    host = run.server(server, fx_name)
    unit = [] if "--unit-id" in args else ["--unit-id", str(fx["unitId"])]
    code, doc, err = run.client(client, operation, "--host", host, *unit, *args)
    if check == "fixture":
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
    if check == "refused":
        return None if code != 0 and doc and not doc["ok"] else f"exit {code}: {doc}"
    model, name, value = flag(args, "--model"), args[args.index("--point") + 1], flag(args, "--value")
    if check == "readback":
        if code != 0 or doc["result"]["value"] != value:
            return f"exit {code}: {doc} {err}"
        _, doc, _ = run.client(client, "scan", "--host", host, *unit)
        got = point(doc, model, name)
        return None if got == value else f"read back {got}"
    if check == "refused-unchanged":
        if code != 1 or doc["ok"]:
            return f"a read-only point was written: exit {code}: {doc}"
        _, doc, _ = run.client(client, "scan", "--host", host, *unit)
        want = next(m for m in fx["models"] if m["ID"] == model)[name]
        got = point(doc, model, name)
        return None if got == want else f"value changed to {got}"
    raise ValueError(f"unknown check {check}")


def interop(pairs):
    images = dict(p.split("=", 1) for p in pairs)
    started = ir.now()
    run = Run(images)
    try:
        for server in images:
            if not run.has_role(server, "server"):
                continue
            for client in images:
                if not run.has_role(client, "client"):
                    continue
                roles = {"client": client, "server": server}
                for cid, fx_name, operation, args, check in cases():
                    if not run.has_operation(client, operation):
                        run.record(cid, roles, "unsupported", f"{client} does not declare the client operation {operation}")
                        continue
                    run.case(cid, roles, lambda c=client, s=server, f=fx_name, o=operation, a=args, k=check: device_case(run, c, s, f, o, a, k))
    finally:
        run.close()
    implementations = []
    for name, image in images.items():
        up = run.caps[name]["upstream"]
        i = {"name": name, "version": up["version"], "independent": up.get("independentEngine", True) is not False}
        if digest := ir.repo_digest(image):
            i["image"] = digest
        implementations.append(i)
    doc = {
        "schemaVersion": "1.0", "repository": "otfabric/sunspec-interop",
        "interopVersion": ir.version_field(str(ROOT), "interop_version"),
        "contractVersion": ir.version_field(str(ROOT), "contract_version"),
        "gitSha": ir.git_sha(str(ROOT)), "startedAt": started, "finishedAt": ir.now(), "host": ir.host(),
        "implementations": implementations,
        "reproduce": f"REPORT_DIR=reports VERSION={os.environ.get('VERSION', 'dev')} scripts/interop.sh {' '.join(images)}",
        "results": run.results,
    }
    path = run.reports / "results.json"
    ir.write(str(path), doc)
    bad = ["schema: " + e for e in ir.schema_errors(doc)] + ir.problems(doc, FLOOR)
    for b in bad:
        print("FAIL " + b, file=sys.stderr)
    print(f"[interop] {doc['summary']['pass']} passed, {len(doc['results']) - doc['summary']['pass']} other")
    return 1 if bad else 0


def main(argv):
    if len(argv) == 3 and argv[0] == "smoke":
        return smoke(argv[1], argv[2])
    if len(argv) >= 2 and argv[0] == "interop" and all("=" in a for a in argv[1:]):
        return interop(argv[1:])
    print(__doc__, file=sys.stderr)
    return 2


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
