# sunspec-interop Releases

The `sunspec-interop-otfabric` image is not part of these releases. It has its own
manual workflow and no releases (`own/go-sunspec/README.md`).

## v0.2.0

**Date:** 2026-11-06
**Previous release:** v0.1.0

### Added

- Container contract 1.1: `upstream.independentEngine` in the capability document (`true` for pysunspec2; absent in a 1.0 image means `true`), and a client operation an image does not list is a usage error.
- `own/go-sunspec/` and the manual **Own stack (go-sunspec)** workflow: OT Fabric's go-sunspec behind the contract as `ghcr.io/otfabric/sunspec-interop-otfabric`, built from the latest go-sunspec release, for others to test against. It is not a reference and not in this release or `manifest.json` (otfabric/engineering ADR-0012).
- `scripts/smoke.sh <adapter>` and `scripts/interop.sh <adapters>`, with `scripts/lib.sh`: an image comes from `ADAPTER_IMAGE_<adapter>` when it is set. The cross-stack test runs every client against every server on the cases of `tests/cases.tsv`; a client operation an image does not declare is `unsupported`, and `results.json` takes `independent` from the image.
- `REQUIREMENTS.md` and `compose.yaml`, as in the other harnesses.

### Changed

- The smoke test also checks the baked fixtures, the user, the health check and an undeclared operation. `make smoke` and `make interop` are separate targets; the pass floor of a run is 6.
- CI's R1 check also refuses go-modbus and modbusctl outside `own/`.

The image `sunspec-interop-pysunspec2` is still pysunspec2 1.3.7.

---
## v0.1.0

**Date:** 2026-10-31
**Previous release:** none

First release.

- Image `sunspec-interop-pysunspec2` with pysunspec2 1.3.7: a device whose registers pysunspec2 encodes, and a client that scans and writes.
- Container contract 1.0, fixtures `baseline`, `edge` and `altbase`.
- Self-test with interop-result-v1 results, native builds on amd64 and arm64, daily upstream check.

---
