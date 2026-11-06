# Third-party notices

The images published from this repository contain third-party software.
Every image carries the licence texts of what it contains under `/licenses`.

## What is licensed how

| Part | Licence |
|------|---------|
| Repository tooling, fixtures, schemas, scripts, documentation | MIT ([LICENSE](LICENSE)) |
| Adapter programs under `adapters/` and `own/` | MIT |
| The published images | the adapter (MIT) together with the components listed below, each under its own licence |

Nothing in this repository is linked into a consumer: a library that tests
against the images talks to them over TCP and reads their output, and is not
affected by their licences.

## Image `sunspec-interop-otfabric`

Built by hand from `own/go-sunspec/` (not part of a release); every module's licence is in the image under `/licenses`.

| Component | Version | Licence | Source |
|-----------|---------|---------|--------|
| go-sunspec | latest release when built | MIT | https://github.com/otfabric/go-sunspec |
| go-modbus | as go-sunspec requires | MIT | https://github.com/otfabric/go-modbus |
| go-serial | as go-modbus requires | MIT | https://github.com/otfabric/go-serial |
| Go standard library | 1.27.2 | BSD-3-Clause | https://go.dev |
| Adapter | this repository, `own/go-sunspec/` | MIT | https://github.com/otfabric/sunspec-interop |
| Base system | Debian trixie-slim | various; see `/usr/share/doc/*/copyright` in the image | https://www.debian.org |

## Image `sunspec-interop-cbsigma`

| Component | Version | Licence | Source |
|-----------|---------|---------|--------|
| cB-Sigma sunspec (Abhinav Gautam), the client | v0.3.0, commit `c99cbef0ba8fefb8433663db152f9fdde8c7adb1` | Apache-2.0 | https://github.com/cB-Sigma/sunspec |
| cB-Sigma sunspec-sim (Abhinav Gautam), the device | v0.3.0, commit `e407ee734647776195ce40fe6bdbffcb8ee05d6e` | Apache-2.0 | https://github.com/cB-Sigma/sunspec-sim |
| simonvetter/modbus, linked by cB-Sigma sunspec | v1.6.4 | MIT | https://github.com/simonvetter/modbus |
| goburrow/serial, linked by cB-Sigma sunspec | v0.1.0 | MIT | https://github.com/goburrow/serial |
| Go standard library and runtime | 1.27.2 | BSD-3-Clause | https://go.dev |
| Adapter | this repository, `adapters/cbsigma/` | MIT | https://github.com/otfabric/sunspec-interop |
| Base system | Debian trixie-slim | various; see `/usr/share/doc/*/copyright` in the image | https://www.debian.org |

Licences and notices in the image: `/licenses/sunspec/` and `/licenses/sunspec-sim/`
(`LICENSE`, `NOTICE`), `/licenses/modbus/LICENSE.txt`, `/licenses/serial/LICENSE`,
`/licenses/go/LICENSE`. cB-Sigma sunspec embeds the SunSpec model definitions it
uses. Both cB-Sigma modules are published as git tags, without GitHub releases.

## Image `sunspec-interop-pysunspec2`

| Component | Version | Licence | Source |
|-----------|---------|---------|--------|
| pysunspec2 | 1.3.7 (wheel `sha256:4bc5029ac2ee9ec8ea7bf7e4623c0fc269244c2f0292b2b7dca8588a4287eeeb`) | Apache-2.0 | https://github.com/sunspec/pysunspec2 |
| pyserial | 3.5 (wheel `sha256:c4451db6ba391ca6ca299fb3ec7bae67a5c55dde170964c7a14ceefec02f2cf0`) | BSD-3-Clause | https://github.com/pyserial/pyserial |
| CPython | 3.14.8 | PSF-2.0 | https://www.python.org |
| Adapter | this repository, `adapters/pysunspec2/` | MIT | https://github.com/otfabric/sunspec-interop |
| Base system | Debian trixie-slim (python image) | various; see `/usr/share/doc/*/copyright` in the image | https://www.debian.org |

The pysunspec2 wheel includes the SunSpec model definitions it uses. The pyserial
wheel has no licence file, so `adapters/pysunspec2/licenses/pyserial-LICENSE.txt`
is `LICENSE.txt` of the pyserial 3.5 source distribution
(`sha256:3c77e014170dfffbd816e6ffc205e9842efb10be9f58ec16d3e8675b4925cddb`).
