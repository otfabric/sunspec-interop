#!/usr/bin/env bash
# Cross-stack self-test: every client adapter against every server adapter.
#
# Usage: interop.sh <adapter> [<adapter> ...]      e.g. interop.sh pysunspec2
#
# Runs every case of tests/cases.tsv for each client/server pairing, the same
# adapter on both sides included: scans that must read every fixture as it is
# written, a write of a read-write point, a refused write of a read-only one
# and a request for another unit. A client that does not declare an operation
# is "unsupported" for its cases, never a pass.
#
# This tests the adapters against each other. It says nothing about any
# library that consumes the images.
#
# Environment: REGISTRY, VERSION (image tag, default dev), ADAPTER_IMAGE_<adapter>,
#              REPORT_DIR (keeps every result document when set, and writes
#              results.json, interop-result-v1: only a pass counts as passing).

set -uo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/lib.sh
source "${SCRIPT_DIR}/lib.sh"

[[ $# -ge 1 ]] || { echo "usage: interop.sh <adapter> [<adapter> ...]" >&2; exit 2; }
require_tools
pairs=()
for adapter in "$@"; do
    pairs+=("${adapter}=$(image_of "${adapter}")")
done
log "adapters: $* (${VERSION})"
exec python3 "${SCRIPT_DIR}/selftest.py" interop "${pairs[@]}"
