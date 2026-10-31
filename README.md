# sunspec-interop

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![CI Status](https://github.com/otfabric/sunspec-interop/actions/workflows/ci.yml/badge.svg)](https://github.com/otfabric/sunspec-interop/actions/workflows/ci.yml)
[![Latest Release](https://img.shields.io/github/v/release/otfabric/sunspec-interop?label=release)](https://github.com/otfabric/sunspec-interop/releases/latest)

An independent SunSpec implementation in a container, behind a documented command-line contract, with deterministic fixtures. Protocol libraries use it as an oracle: the SunSpec Alliance's [pysunspec2](https://github.com/sunspec/pysunspec2) reads what a library serves, and serves what a library reads.

This is test equipment. It is not a SunSpec implementation of its own, and a pass here is not SunSpec certification.

| Image | Stack | Roles |
|-------|-------|-------|
| `ghcr.io/otfabric/sunspec-interop-pysunspec2` | pysunspec2 1.3.7 (Python) | device, client |

pysunspec2 has no Modbus server. In the device role pysunspec2 encodes the register map from a fixture, and a small Modbus TCP layer of the adapter serves those registers. The SunSpec encoding is pysunspec2's; the transport is not, and is not what is under test ([docs/CAPABILITIES.md](docs/CAPABILITIES.md)).

## Quick start

```sh
make images interop                 # build, then self-test
docker run --rm ghcr.io/otfabric/sunspec-interop-pysunspec2:dev print-capabilities
make run-pysunspec2                 # a device on 127.0.0.1:1502
```

| Document | Content |
|----------|---------|
| [docs/CONTAINER_CONTRACT.md](docs/CONTAINER_CONTRACT.md) | Commands, documents, exit codes |
| [docs/FIXTURES.md](docs/FIXTURES.md) | What a device serves |
| [docs/CAPABILITIES.md](docs/CAPABILITIES.md) | What the stack does and does not do |
| [docs/COMPATIBILITY.md](docs/COMPATIBILITY.md) | Results and known differences |
| [TESTING.md](TESTING.md) | The self-test and how to reproduce it |
| [versions.yaml](versions.yaml) | Pins |
| [RELEASE.md](RELEASE.md) | Releases |

## Licence

MIT for this repository. The images contain third-party software: [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).
