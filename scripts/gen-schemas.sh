#!/usr/bin/env bash
# Copyright Mondoo, Inc. 2026
# SPDX-License-Identifier: Apache-2.0
#
# Regenerates the embedded MQL resource schemas (internal/engine/schemas/) from
# the exact mql version pinned in go.mod, so the schemas always match the
# providers compiled into skillcheck.
#
# mql does not commit *.resources.json; they are generated from the .lr files
# with mql's mqlr tool. mqlr also writes Go code next to each .lr file, and the
# Go module cache is read-only, so the provider resource directories are copied
# into a temp dir and generated there. os.lr imports core and network, so
# those are copied too.
#
# Usage: scripts/gen-schemas.sh [dest]   (default: internal/engine/schemas)

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
dest="${1:-$repo_root/internal/engine/schemas}"

cd "$repo_root"
go mod download go.mondoo.com/mql
mql_dir="$(go list -m -f '{{.Dir}}' go.mondoo.com/mql)"
mql_version="$(go list -m -f '{{.Version}}' go.mondoo.com/mql)"
echo "Generating schemas from go.mondoo.com/mql ${mql_version}"

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

(cd "$mql_dir" && go build -o "$work/mqlr" ./providers-sdk/v1/mqlr)

for provider in os core network; do
  mkdir -p "$work/src/providers/$provider"
  cp -R "$mql_dir/providers/$provider/resources" "$work/src/providers/$provider/"
done
chmod -R u+w "$work/src"

mkdir -p "$work/out"
for provider in os core; do
  (cd "$work/src" && "$work/mqlr" go "providers/$provider/resources/$provider.lr" --dist "$work/out" >/dev/null 2>"$work/mqlr.log") || {
    cat "$work/mqlr.log" >&2
    echo "mqlr failed for $provider" >&2
    exit 1
  }
done

mkdir -p "$dest"
cp "$work/out/os.resources.json" "$work/out/core.resources.json" "$dest/"
echo "Wrote $dest/os.resources.json and $dest/core.resources.json"
