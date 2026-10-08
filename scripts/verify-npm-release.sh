#!/usr/bin/env bash
# Copyright Mondoo, Inc. 2026
# SPDX-License-Identifier: Apache-2.0
#
# Verifies that an npm release is live and installable after `npm publish`
# claims it shipped.
#
# Phase 1 polls the registry until all seven packages list the version.
# `npm publish` returning 0 means npm accepted the tarball, not that it is
# visible yet, so this is a poll to a deadline rather than a one-shot check.
#
# Phase 2 installs @mondoohq/skillcheck@<version> into a clean directory and
# checks the exit contract through the npm launcher, which is what npm users
# actually run: `validate` passes on a good repo (0) and fails on a bad one (1).
#
# Usage: verify-npm-release.sh <version>
#   VERIFY_NPM_DEADLINE        seconds to wait for propagation (default 3600)
#   VERIFY_NPM_INTERVAL        seconds between polls (default 30)
#   VERIFY_NPM_SKIP_INSTALL=1  registry phase only

set -euo pipefail

version="${1:-}"
if [ -z "$version" ]; then
  echo "usage: $0 <version>" >&2
  exit 2
fi
version="${version#v}"
if [[ ! "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]]; then
  echo "version '${version}' is not a semver version" >&2
  exit 2
fi

deadline_secs="${VERIFY_NPM_DEADLINE:-3600}"
interval="${VERIFY_NPM_INTERVAL:-30}"
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# The package list comes from the module that builds the packages, so a new
# platform cannot be added without this check covering it.
packages=()
while IFS= read -r line; do
  [ -n "$line" ] && packages+=("$line")
done < <(cd "$repo_root" && node --input-type=module -e "
  import { PLATFORMS, WRAPPER_NAME, packageNameFor } from './scripts/npm-package.mjs';
  for (const p of PLATFORMS) console.log(packageNameFor(p));
  console.log(WRAPPER_NAME);
")
if [ "${#packages[@]}" -ne 7 ]; then
  echo "expected 7 npm packages from scripts/npm-package.mjs, got ${#packages[@]}" >&2
  exit 1
fi
echo "Verifying ${#packages[@]} packages at version ${version}"

# --- Phase 1: every package is on the registry at this version ---------------
deadline=$(( $(date +%s) + deadline_secs ))
pending=("${packages[@]}")
while :; do
  still=()
  for pkg in "${pending[@]}"; do
    url="https://registry.npmjs.org/${pkg/\//%2f}"
    # The version reaches node through the environment, never the program text.
    if curl --fail --silent --show-error --location --max-time 20 "$url" 2>/dev/null \
      | VERIFY_VERSION="$version" node -e '
          let s = "";
          process.stdin.on("data", (c) => (s += c)).on("end", () => {
            try {
              const d = JSON.parse(s);
              process.exit(d.versions && d.versions[process.env.VERIFY_VERSION] ? 0 : 1);
            } catch {
              process.exit(1);
            }
          });'; then
      echo "  ok      ${pkg}@${version}"
    else
      still+=("$pkg")
    fi
  done
  [ "${#still[@]}" -eq 0 ] && break
  pending=("${still[@]}")

  if [ "$(date +%s)" -ge "$deadline" ]; then
    echo >&2
    echo "after ${deadline_secs}s these are still not on the registry at ${version}:" >&2
    printf '  %s\n' "${pending[@]}" >&2
    exit 1
  fi
  echo "  waiting on ${#pending[@]} package(s); retrying in ${interval}s"
  sleep "$interval"
done
echo "All ${#packages[@]} packages are live."

if [ "${VERIFY_NPM_SKIP_INSTALL:-}" = "1" ]; then
  exit 0
fi

# --- Phase 2: a real install runs, and exit codes survive the launcher -------
workdir="$(mktemp -d)"
trap 'rm -rf "$workdir"' EXIT
cd "$workdir"

echo "Installing @mondoohq/skillcheck@${version}"
npm install --prefer-online --no-audit --no-fund --silent "@mondoohq/skillcheck@${version}"

launcher="node_modules/@mondoohq/skillcheck/bin/skillcheck.js"
if [ ! -f "$launcher" ]; then
  echo "the wrapper did not install its launcher at ${launcher}" >&2
  ls -R node_modules/@mondoohq >&2 || true
  exit 1
fi

reported="$(node "$launcher" --version)"
echo "  version: ${reported}"
case "$reported" in
  *" ${version} "*) ;;
  *)
    echo "installed skillcheck reports '${reported}', want version ${version}" >&2
    exit 1
    ;;
esac

set +e
node "$launcher" validate --no-color "${repo_root}/internal/validate/testdata/good" >/dev/null 2>&1
good_rc=$?
node "$launcher" validate --no-color "${repo_root}/internal/validate/testdata/bad" >/dev/null 2>&1
bad_rc=$?
set -e

echo "  validate good exit=${good_rc}, validate bad exit=${bad_rc}"
if [ "$good_rc" -ne 0 ] || [ "$bad_rc" -ne 1 ]; then
  echo "exit contract broken through the npm launcher: want good=0 bad=1" >&2
  exit 1
fi
echo "npm release ${version} verified."
