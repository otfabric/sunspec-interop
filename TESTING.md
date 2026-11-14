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

This repository tests itself with two independent stacks, pysunspec2 and cB-Sigma's, each as client and device against both, so it shows that the adapters and the contract work and that the two stacks agree, not that either is right. Evidence about another implementation comes from that implementation's own suite against these images.

## When CI runs

| Event | Workflow | What runs |
|---|---|---|
| Pull request | `pr.yml` | `policy.yml` (otfabric-policy) always; `ci.yml` unless every changed file is documentation; then `OT Fabric Required Checks` |
| Push to a branch | none | nothing; dispatch `ci.yml` to test a branch without a pull request |
| Push to `main` | `policy.yml` | the light checks; the merged commit was tested as the pull request's merge commit |
| Schedule | `ci.yml`, `upstream.yml` | the scheduled runs, on `main` |
| Release | `release.yml` | the release workflow tests the release commit again |

Documentation means an added or modified `*.md` file outside `.github/`, `testdata/` and `vendor/`, other than `AGENTS.md` and `CLAUDE.md`. A deleted or renamed document, or any other file, runs everything. `OT Fabric Required Checks` fails unless every job that applies succeeded; its summary lists them.
