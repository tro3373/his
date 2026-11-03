#!/usr/bin/env bash
# Runs `his gen` (catch-up mode, up to yesterday) built from a base ref and from
# the work tree, and diffs what each wrote. Two passes: empty log dirs, so every
# day is planned and rendered from scratch, and copies of the real logs, so the
# merge into existing days runs.
# Usage: scripts/gen-equiv.sh [base-ref]   (default: HEAD)
set -euo pipefail

base=${1:-HEAD}
repo=$(git rev-parse --show-toplevel)
work=$(mktemp -d)
trap 'trash "$work"' EXIT

git -C "$repo" worktree add --quiet --detach "$work/base-src" "$base"
(cd "$work/base-src" && go build -o "$work/his-base" .)
git -C "$repo" worktree remove --force "$work/base-src"
(cd "$repo" && go build -o "$work/his-new" .)

run() {
  local bin=$1 pass=$2
  local out=$work/$pass/$bin
  mkdir -p "$out"
  yq -o yaml ".gen.logs[] |= (.dir as \$d | .dir = \"$out/\" + (\$d | split(\"/\") | .[-1]))" ~/.his >"$out.yaml"
  if [[ $pass == copy ]]; then
    yq -r '.gen.logs[].dir' ~/.his | while read -r d; do
      d=${d/#\~/$HOME}
      cp -r "$d" "$out/$(basename "$d")"
    done
  fi
  "$work/his-$bin" --config "$out.yaml" gen | sed "s#$out#X#" >"$out.log"
}

for pass in empty copy; do
  run base "$pass"
  run new "$pass"
  diff -u "$work/$pass/base.log" "$work/$pass/new.log"
  diff -r "$work/$pass/base" "$work/$pass/new"
  echo "==> $pass: equivalent, $(wc -l <"$work/$pass/base.log") writes by both"
done
