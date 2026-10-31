# AGENTS.md — sunspec-interop

## What this is

The SunSpec Alliance's pysunspec2 in a container behind one command-line contract, with deterministic fixtures, used as an oracle by SunSpec libraries. Nothing here depends on a library under test. Profile: **interoperability**.

## Organisation standards

This repository follows **otfabric/engineering policy 1.2**, profile **interoperability** (`.otfabric/policy.yaml`, mode `enforce`). Where this file and the organisation standards disagree, this file wins here — say so in your report.

## Where the truth lives

| Question | Source |
|---|---|
| Container commands, documents, exit codes | docs/CONTAINER_CONTRACT.md (`contract_version` in versions.yaml) |
| What the stack can do | docs/CAPABILITIES.md |
| Pins (upstream version, wheel hashes, base image digest) | versions.yaml, checked by `make validate` |
| Fixtures | fixtures/, docs/FIXTURES.md |
| Expected deviations of upstream versions | tests/known-bugs.tsv |
| Results | docs/COMPATIBILITY.md |

## Hard rules

1. Nothing floats: upstream wheels and base images stay pinned, and versions.yaml mirrors the pins. `make validate` must pass.
2. Skips and known differences are never counted as passes.
3. An adapter refuses an unsupported operation with a usage error, never with a fake success.
4. Contract changes bump `contract_version` and reach docs/CONTAINER_CONTRACT.md in the same change.
5. Register encoding and decoding stay pysunspec2's. The adapter's code carries Modbus frames and nothing about SunSpec layout.
6. Licences: every redistributed component in THIRD_PARTY_NOTICES.md.
7. The release tag equals `interop_version`; bump versions.yaml before tagging.

## Commands

| When you change… | Run |
|---|---|
| anything | `make validate lint` |
| the adapter or fixtures | `make image-pysunspec2 interop` |
| documents or the contract | `make documents` |

## Git and releases

Branches and PRs. Do not push to `main`, push tags or publish images unless the maintainer asks in the session.
