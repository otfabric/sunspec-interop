# Testing

| Command | What it runs | Needs |
|---------|--------------|-------|
| `make validate` | versions.yaml against the Dockerfile, RELEASE.md, fixtures against the schema, unit tests of the result scripts | Python 3 |
| `make lint` | Shellcheck | shellcheck or Docker |
| `make interop` | `scripts/selftest.py`: the contract, then the client against the device on every fixture | Docker, a built image |
| `make documents` | The same, keeping every document in `reports/`, validated against `schemas/` | Docker |
| `make ci` | All of the above | Docker |

The self-test writes `results.json` in the interop-result-v1 format. A run fails on any `fail` or `infra-error`, and with fewer than 8 passes.

This repository tests itself with one stack on both sides, so it shows that the adapter and the contract work, not that pysunspec2 is right. Evidence about another implementation comes from that implementation's own suite against these images.
