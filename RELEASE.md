# sunspec-interop Releases

The `sunspec-interop-otfabric` image is not part of these releases. It has its own
manual workflow and no releases (`own/go-sunspec/README.md`).

## v0.3.0

**Date:** 2026-11-06
**Previous release:** v0.2.0

### Added

- Image `sunspec-interop-cbsigma`: cB-Sigma's Go SunSpec stack as a second independent reference. The client is [cB-Sigma sunspec](https://github.com/cB-Sigma/sunspec) v0.3.0 (commit `c99cbef`), the device [cB-Sigma sunspec-sim](https://github.com/cB-Sigma/sunspec-sim) v0.3.0 (commit `e407ee7`), both Apache-2.0 and pinned by tag (the project publishes tags, no GitHub releases). `adapters/cbsigma/go.mod` is the source of truth for the build; `scripts/check-versions.sh` checks it against the Dockerfile and versions.yaml.
- Container contract 1.2: the client operation `write-group` writes a point of a group, named by its path (`PFWInj.PF`, `Crv[1].Pt[0].V`). pysunspec2 and cbsigma declare it; the otfabric image does not (go-sunspec writes top-level points only), so its cases are `unsupported` for that client.
- Fixture `der` (unit 3, base 40000): IEEE 1547-2018 models 701, 702, 704 (groups that occur once) and 705 (a volt-var curve set: repeating curves, each with repeating points counted by a top-level point).
- Cases: `device.scan.der`; `device.scale.edge` and `device.scale.der` check engineering values (scale factors from the top level applied in nested groups); `der.write.*` write DER settings and a signed setpoint and refuse a read-only status; `der.write-group.*` write a control group point, a curve point and a curve setting, and refuse the read-only curve flag.
- The daily upstream check also follows cB-Sigma sunspec's tags.

### Changed

- The scan document holds a group that occurs once as an object (the contract said nothing about it; pysunspec2 already did so).
- The own-stack workflow and `own/go-sunspec/Makefile` test the otfabric image against both references.
- `own/go-sunspec/` (not in this release): contract 1.2 without `write-group`; its scan document holds a group that occurs once as an object, where it held a one-element list (`device.scan.der` caught it; go-sunspec decoded the group right).
- The self-test records a client that prints no document as a failure instead of stopping the run.

The image `sunspec-interop-pysunspec2` is still pysunspec2 1.3.7; its adapter adds `write-group`.

---
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
