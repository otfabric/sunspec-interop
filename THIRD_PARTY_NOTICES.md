# Third-party notices

The images published from this repository contain third-party software.
Every image carries the licence texts of what it contains under `/licenses`.

## What is licensed how

| Part | Licence |
|------|---------|
| Repository tooling, fixtures, schemas, scripts, documentation | MIT ([LICENSE](LICENSE)) |
| Adapter programs under `adapters/` | MIT |
| The published images | the adapter (MIT) together with the components listed below, each under its own licence |

Nothing in this repository is linked into a consumer: a library that tests
against the images talks to them over TCP and reads their output, and is not
affected by their licences.

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
