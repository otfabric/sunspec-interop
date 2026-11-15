#!/usr/bin/env bash
# Fails when versions.yaml disagrees with what the builds actually pin.
#
# Every adapter Dockerfile declares its pins as build arguments with a default:
#   ARG UPSTREAM_VERSION=...     the upstream release
#   ARG UPSTREAM_SHA=...         its commit, where the build fetches one
#   ARG <NAME>_DIGEST=sha256:... every base image
# Each of those values must appear in versions.yaml. Besides:
#   - every implementation names the commit of its version (revision) and a
#     lock file of the repository that pins that version (lock); a
#     requirements.txt lock has the wheel hashes versions.yaml records
#     (wheel_sha256, and the one of each bundled package);
#   - every base image digest of an own/<stack>/Dockerfile is in base_images
#     with the own_implementations entry of "build: own/<stack>" in used_by,
#     and every base image used_by that entry is in the Dockerfile.
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT}"
status=0

need() {  # need <description> <value that must appear in versions.yaml>
    if grep -qF -- "$2" versions.yaml; then
        echo "ok   $1: $2"
    else
        echo "FAIL $1: $2 is not in versions.yaml"
        status=1
    fi
}
ok()  { echo "ok   $*"; }
bad() { echo "FAIL $*"; status=1; }

# field <section> <name> <key>: a scalar of versions.yaml, without quotes and
# without a trailing comment.
field() {
    awk -v sec="$1:" -v name="  $2:" -v key="    $3:" '
        $0 == sec { in_sec = 1; next }
        /^[a-z_]+:/ { in_sec = 0 }
        in_sec && /^  [a-z0-9-]+:$/ { in_name = ($0 == name); next }
        in_sec && in_name && index($0, key) == 1 {
            v = substr($0, length(key) + 1); sub(/ +#.*$/, "", v); gsub(/^ +| +$|"/, "", v); print v; exit
        }' versions.yaml
}
# names <section>: the entries of a section.
names() { awk -v sec="$1:" '$0 == sec { s = 1; next } /^[a-z_]+:/ { s = 0 } s && /^  [a-z0-9-]+:$/ { sub(/:$/, ""); print $1 }' versions.yaml; }
# entry <section> <name>: the lines of one entry of a section.
entry() {
    awk -v sec="$1:" -v name="  $2:" '
        $0 == sec { in_sec = 1; next }
        /^[a-z_]+:/ { in_sec = 0 }
        in_sec && /^  [a-z0-9-]+:$/ { in_name = ($0 == name); next }
        in_sec && in_name { print }' versions.yaml
}

found=0
for dockerfile in adapters/*/Dockerfile; do
    [[ -f "${dockerfile}" ]] || continue
    found=$((found + 1))
    adapter=$(basename "$(dirname "${dockerfile}")")
    version=$(sed -nE 's/^ARG UPSTREAM_VERSION=(.+)$/\1/p' "${dockerfile}" | head -1)
    if [[ -z "${version}" ]]; then
        echo "FAIL ${adapter}: the Dockerfile has no ARG UPSTREAM_VERSION=<pin>"
        status=1
    else
        need "${adapter} version" "version: \"${version}\""
    fi
    sha=$(sed -nE 's/^ARG UPSTREAM_SHA=(.+)$/\1/p' "${dockerfile}" | head -1)
    [[ -n "${sha}" ]] && need "${adapter} revision" "${sha}"
    while read -r name digest; do
        need "${adapter} ${name}" "${digest}"
    done < <(sed -nE 's/^ARG ([A-Z0-9_]+_DIGEST)=(sha256:[0-9a-f]{64})$/\1 \2/p' "${dockerfile}")
    if grep -nE '^FROM ' "${dockerfile}" | grep -vE '@(\$\{[A-Z0-9_]+_DIGEST\}|sha256:)| AS [a-z]+$|^[0-9]+:FROM [a-z]+( AS [a-z]+)?$' >/dev/null; then
        echo "FAIL ${adapter}: a FROM line is not pinned by digest"
        status=1
    fi
done
# cbsigma: two upstream modules, both pinned in the Dockerfile and in go.mod
# (adapters/cbsigma/go.mod is the source of truth for a build).
if [[ -f adapters/cbsigma/Dockerfile ]]; then
    arg() { sed -nE "s/^ARG $2=(.+)$/\1/p" "$1" | head -1; }
    gomod() { sed -nE "s|^[[:space:]]*(require[[:space:]]+)?$1 (v[^ ]+).*$|\2|p" adapters/cbsigma/go.mod; }
    for pair in "UPSTREAM_VERSION github.com/cB-Sigma/sunspec" "SIM_VERSION github.com/cB-Sigma/sunspec-sim"; do
        read -r name module <<<"${pair}"
        pinned="$(arg adapters/cbsigma/Dockerfile "${name}")"
        if [[ -n "${pinned}" && "${pinned}" == "$(gomod "${module}")" ]]; then
            echo "ok   cbsigma ${module} in Dockerfile and go.mod: ${pinned}"
        else
            echo "FAIL cbsigma ${module}: Dockerfile ${name}=${pinned}, go.mod $(gomod "${module}")"
            status=1
        fi
    done
    need "cbsigma sunspec-sim revision" "$(arg adapters/cbsigma/Dockerfile SIM_SHA)"
    need "cbsigma sunspec-sim module" "github.com/cB-Sigma/sunspec-sim $(arg adapters/cbsigma/Dockerfile SIM_VERSION)"
fi
for name in $(names implementations); do
    version=$(field implementations "${name}" version)
    revision=$(field implementations "${name}" revision)
    if [[ "${revision}" =~ ^[0-9a-f]{40}$ ]]; then ok "${name}: revision ${revision:0:12}"
    else bad "${name}: revision '${revision}' is not a commit (40 hexadecimal digits)"; fi
    lock=$(field implementations "${name}" lock)
    if [[ -z "${lock}" || ! -f "${lock}" ]]; then
        bad "${name}: lock '${lock}' is not a file of this repository"
        continue
    elif grep -qwF -- "${version}" "${lock}"; then
        ok "${name}: ${lock} pins ${version}"
    else
        bad "${name}: ${lock} does not pin version ${version}"
    fi
    [[ "${lock}" == */requirements.txt ]] || continue
    # requirements.txt: "<package>==<version> \" and then "--hash=sha256:<hash>".
    pinned=$(entry implementations "${name}")
    while read -r package pkgversion hash; do
        if [[ "${package}" == "${name}" ]]; then
            if [[ "${pkgversion}" == "${version}" && "${hash}" == "$(field implementations "${name}" wheel_sha256)" ]]; then
                ok "${name}: ${lock} has the wheel_sha256 of ${package} ${pkgversion}"
            else
                bad "${name}: ${lock} has ${package} ${pkgversion} sha256:${hash}, versions.yaml ${version} wheel_sha256 $(field implementations "${name}" wheel_sha256)"
            fi
        elif grep -E "^      ${package}: \"${pkgversion//./\\.}\"" <<< "${pinned}" | grep -qF -- "${hash}"; then
            ok "${name}: ${lock} has the bundled ${package} ${pkgversion} of versions.yaml, with its hash"
        else
            bad "${name}: ${lock} has ${package} ${pkgversion} sha256:${hash}, which is not a bundled package of versions.yaml with that version and hash"
        fi
    done < <(awk '/^[A-Za-z0-9_.-]+==[^ ]+/ { split($1, p, "=="); pkg = p[1]; ver = p[2]; next }
                  pkg != "" && match($0, /--hash=sha256:[0-9a-f]{64}/) { print pkg, ver, substr($0, RSTART + 14, 64); pkg = "" }' "${lock}")
done

for dockerfile in own/*/Dockerfile; do
    [[ -f "${dockerfile}" ]] || continue
    dir=${dockerfile%/Dockerfile}
    own=""
    for name in $(names own_implementations); do
        [[ "$(field own_implementations "${name}" build)" == "${dir}" ]] && own=${name}
    done
    if [[ -z "${own}" ]]; then bad "${dockerfile}: no own_implementations entry has build: ${dir}"; continue; fi
    built=$(sed -nE 's/^ARG [A-Z0-9_]+_DIGEST=(sha256:[0-9a-f]{64})$/\1/p' "${dockerfile}")
    listed=""
    for image in $(names base_images); do
        used_by=" $(field base_images "${image}" used_by | tr -d '[],') "
        [[ "${used_by}" == *" ${own} "* ]] && listed+="$(field base_images "${image}" digest)"$'\n'
    done
    while read -r digest; do
        [[ -n "${digest}" ]] || continue
        if grep -qxF -- "${digest}" <<< "${listed}"; then ok "${dockerfile}: ${digest:0:19}… in base_images, used by ${own}"
        else bad "${dockerfile}: base image ${digest} is not in base_images with ${own} in used_by"; fi
    done <<< "${built}"
    while read -r digest; do
        [[ -n "${digest}" ]] || continue
        grep -qxF -- "${digest}" <<< "${built}" || bad "base_images: ${digest} is used_by ${own} but is not a base image of ${dockerfile}"
    done <<< "${listed}"
done

if [[ "${found}" -eq 0 ]]; then
    echo "FAIL no adapter Dockerfile found"
    status=1
fi
exit "${status}"
