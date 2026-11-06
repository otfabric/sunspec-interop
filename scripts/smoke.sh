#!/usr/bin/env bash
# Container-contract smoke test for one adapter image.
#
# Usage: smoke.sh <adapter>          e.g. smoke.sh pysunspec2
#
# Checks the parts of docs/CONTAINER_CONTRACT.md that do not need a second
# adapter: capabilities, the baked fixtures and register maps, usage errors,
# an undeclared operation, an invalid fixture, readiness, the user and the
# health check, and a connection that cannot be made.
#
# Environment: REGISTRY, VERSION (image tag, default dev), ADAPTER_IMAGE_<adapter>,
#              REPORT_DIR (keeps the capability and client documents when set).

set -uo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/lib.sh
source "${SCRIPT_DIR}/lib.sh"

ADAPTER="${1:?usage: smoke.sh <adapter>}"
require_tools
IMAGE="$(image_of "${ADAPTER}")"
log "${IMAGE}"
exec python3 "${SCRIPT_DIR}/selftest.py" smoke "${ADAPTER}" "${IMAGE}"
