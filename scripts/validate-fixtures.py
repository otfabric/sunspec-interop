#!/usr/bin/env python3
# SPDX-License-Identifier: MIT
"""Validates fixtures/ against schemas/fixture.schema.json.

    validate-fixtures.py [--require-schema]

Without the jsonschema package the schema check is skipped, unless
--require-schema makes that an error.
"""

import json
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent


def main(argv):
    try:
        import jsonschema
    except ImportError:
        if "--require-schema" in argv:
            print("FAIL the jsonschema package is not installed")
            return 1
        jsonschema = None
    schema = json.loads((ROOT / "schemas" / "fixture.schema.json").read_text())
    bad = found = 0
    for path in sorted((ROOT / "fixtures").glob("*/fixture.json")):
        found += 1
        doc = json.loads(path.read_text())
        problems = []
        if doc.get("name") != path.parent.name:
            problems.append(f"name {doc.get('name')!r} is not the name of the directory")
        if jsonschema:
            problems += [e.message for e in jsonschema.Draft202012Validator(schema).iter_errors(doc)]
        for p in problems:
            print(f"FAIL {path.parent.name}: {p}")
        bad += len(problems)
    print(f"{found} fixtures, {bad} problems")
    return 1 if bad or not found else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
