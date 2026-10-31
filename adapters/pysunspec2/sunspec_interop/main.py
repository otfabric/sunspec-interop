# SPDX-License-Identifier: MIT
"""Command line of the adapter: docs/CONTAINER_CONTRACT.md."""

import argparse
import asyncio
import json
import sys

from sunspec2.modbus import client as sunspec
from sunspec2.modbus import modbus

from . import client, fixture, server

ADAPTER = "pysunspec2"
CONTRACT = "1.0"
SCHEMA = "1.0"
EXIT_OK, EXIT_FAILED, EXIT_USAGE, EXIT_CONNECT, EXIT_FIXTURE = 0, 1, 2, 3, 4
BUILD_INFO = "/opt/sunspec-interop/build-info.json"


class Parser(argparse.ArgumentParser):
    def error(self, message):
        print(f"usage error: {message}", file=sys.stderr)
        sys.exit(EXIT_USAGE)


def number(text):
    return int(text, 0)


def value(text):
    try:
        return int(text, 0)
    except ValueError:
        return text


def parser():
    p = Parser(prog="sunspec-interop", allow_abbrev=False)
    sub = p.add_subparsers(dest="command", required=True, parser_class=Parser)

    s = sub.add_parser("server", allow_abbrev=False)
    s.add_argument("--fixture", default=fixture.path_of("baseline"))
    s.add_argument("--bind-address", default="0.0.0.0")
    s.add_argument("--port", type=number, default=1502)
    s.add_argument("--ready-file", default="/run/sunspec-interop/ready")

    c = sub.add_parser("client", allow_abbrev=False)
    c.add_argument("operation", choices=sorted(client.OPERATIONS))
    c.add_argument("--host", required=True)
    c.add_argument("--port", type=number, default=1502)
    c.add_argument("--unit-id", type=number, default=1)
    c.add_argument("--timeout", type=float, default=5.0)
    c.add_argument("--model", type=number)
    c.add_argument("--instance", type=number, default=0)
    c.add_argument("--point")
    c.add_argument("--value", type=value)

    sub.add_parser("print-capabilities", allow_abbrev=False)
    for name in ("print-fixture", "print-registers"):
        f = sub.add_parser(name, allow_abbrev=False)
        f.add_argument("name", nargs="?", default="baseline")
    sub.add_parser("version", allow_abbrev=False)
    return p


def build_info():
    try:
        with open(BUILD_INFO, encoding="utf-8") as f:
            return json.load(f)
    except OSError:
        return {"adapterVersion": "dev", "upstreamVersion": "unknown", "upstreamRevision": "unknown", "upstreamArtifact": "unknown"}


def emit(doc):
    print(json.dumps(doc, indent=1, sort_keys=True))


def capabilities():
    info = build_info()
    return {
        "schemaVersion": SCHEMA,
        "adapter": ADAPTER,
        "adapterVersion": info["adapterVersion"],
        "contractVersion": CONTRACT,
        "upstream": {
            "name": "pysunspec2",
            "version": info["upstreamVersion"],
            "revision": info["upstreamRevision"],
            "artifact": info["upstreamArtifact"],
        },
        "roles": ["server", "client"],
        "clientOperations": sorted(client.OPERATIONS),
        "serverFunctions": [3, 6, 16],
        "baseAddresses": [40000, 0, 50000],
        "fixtures": fixture.names(),
    }


def run_client(args):
    doc = {"schemaVersion": SCHEMA, "adapter": ADAPTER, "operation": args.operation, "host": args.host, "port": args.port, "unitId": args.unit_id, "ok": False}
    if args.operation == "write" and (args.model is None or args.point is None or args.value is None):
        print("usage error: write needs --model, --point and --value", file=sys.stderr)
        return EXIT_USAGE
    try:
        doc["result"] = client.OPERATIONS[args.operation](args)
        doc["ok"] = True
        code = EXIT_OK
    except (ConnectionError, TimeoutError, modbus.ModbusClientTimeout) as e:
        doc["error"] = {"kind": "connect", "message": str(e)}
        code = EXIT_CONNECT
    except (modbus.ModbusClientError, sunspec.SunSpecModbusClientError, sunspec.SunSpecModbusValueError, LookupError, OSError) as e:
        kind = "connect" if "connect" in str(e).lower() or "Base address" in str(e) else "failed"
        doc["error"] = {"kind": kind, "message": str(e)}
        code = EXIT_CONNECT if kind == "connect" else EXIT_FAILED
    emit(doc)
    return code


def main(argv):
    args = parser().parse_args(argv)
    try:
        if args.command == "server":
            device = server.Device(fixture.load(args.fixture))
            asyncio.run(server.run(device, args.bind_address, args.port, args.ready_file))
            return EXIT_OK
        if args.command == "client":
            return run_client(args)
        if args.command == "print-capabilities":
            emit(capabilities())
        elif args.command == "print-fixture":
            emit(fixture.load(fixture.path_of(args.name)))
        elif args.command == "print-registers":
            fx = fixture.load(fixture.path_of(args.name))
            data = fixture.image(fixture.build(fx))
            emit({
                "schemaVersion": SCHEMA, "adapter": ADAPTER, "fixture": fx["name"], "unitId": fx["unitId"], "baseAddress": fx["baseAddress"],
                "registers": [int.from_bytes(data[i:i + 2], "big") for i in range(0, len(data), 2)],
            })
        elif args.command == "version":
            info = build_info()
            print(f"sunspec-interop {info['adapterVersion']} (pysunspec2 {info['upstreamVersion']})")
    except fixture.FixtureError as e:
        print(f"fixture error: {e}", file=sys.stderr)
        return EXIT_FIXTURE
    return EXIT_OK
