# Testing

| Command | What it runs | Needs |
|---------|--------------|-------|
| `make validate` | versions.yaml against the Dockerfile, RELEASE.md, fixtures against the schema, unit tests of the result scripts | Python 3 |
| `make lint` | Shellcheck | shellcheck or Docker |
| `make smoke` | `scripts/smoke.sh <adapter>`: the contract of each image on its own | Docker, built images |
| `make interop` | `scripts/interop.sh <adapters>`: every case of `tests/cases.tsv` for every client against every server | Docker, built images |
| `make documents` | The same, keeping every document in `reports/`, validated against `schemas/` | Docker |
| `make ci` | All of the above | Docker |

Both scripts take an image from `ADAPTER_IMAGE_<adapter>` when it is set (`scripts/lib.sh`), and run `scripts/selftest.py`. `interop.sh` writes `results.json` in the interop-result-v1 format, with `independent` taken from the image's `upstream.independentEngine`. A run fails on any `fail` or `infra-error`, and with fewer than 6 passes. A client that does not declare an operation gets `unsupported` for its cases, never a pass.

This repository tests itself with one stack on both sides, so it shows that the adapter and the contract work, not that pysunspec2 is right. Evidence about another implementation comes from that implementation's own suite against these images.
