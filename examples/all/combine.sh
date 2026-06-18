#!/usr/bin/env bash
# Assemble every per-resource and per-data-source example (except the directories
# listed in skip.txt) into this directory as a single Terraform configuration, so
# the combined set is applied/planned/destroyed as one end-to-end test.
#
# Resource examples create and destroy their own resources (named "Production …").
# Data-source examples resolve by a UNIQUE key — a config resource's id/table_name,
# or a permanent uniquely-named fixture seeded in the team ("My Existing …") — never
# a duplicate-prone name, so concurrent or retried runs can't collide or deadlock.
#
# Generated files are named gen_<dir>__<file> and are gitignored. Re-run any
# time the examples change: `make combine`.
set -euo pipefail

ALL="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$ALL/../.." && pwd)"

# Remove previously generated copies so deletions/renames don't linger.
rm -f "$ALL"/gen_*.tf

# Read skip list (directory basenames), ignoring blank lines and comments.
skip=()
if [[ -f "$ALL/skip.txt" ]]; then
  while IFS= read -r line; do
    line="${line%%#*}"
    line="$(echo "$line" | xargs)"
    [[ -n "$line" ]] && skip+=("$line")
  done <"$ALL/skip.txt"
fi

is_skipped() {
  local d="$1"
  for s in ${skip[@]+"${skip[@]}"}; do
    [[ "$s" == "$d" ]] && return 0
  done
  return 1
}

shopt -s nullglob
count=0
for f in "$ROOT"/examples/resources/*/*.tf "$ROOT"/examples/data-sources/*/*.tf; do
  dir="$(basename "$(dirname "$f")")"
  is_skipped "$dir" && continue
  cp "$f" "$ALL/gen_${dir}__$(basename "$f")"
  count=$((count + 1))
done

echo "Assembled $count example file(s) into examples/all/"
