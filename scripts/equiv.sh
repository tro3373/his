#!/usr/bin/env bash
# Builds his from a base ref and from the work tree, runs both against the real
# data of ~/.his and diffs their output.
# - latest and tag read the logs as they are
# - gen runs in catch-up mode (up to yesterday) in two passes: empty log dirs,
#   so every day is planned and rendered from scratch, and copies of the real
#   logs, so the merge into existing days runs
# Usage: scripts/equiv.sh [base-ref]   (default: HEAD)

build() {
  local repo=$1 base=$2 work=$3
  git -C "$repo" worktree add --quiet --detach "$work/base-src" "$base"
  (cd "$work/base-src" && go build -o "$work/his-base" .)
  git -C "$repo" worktree remove --force "$work/base-src"
  (cd "$repo" && go build -o "$work/his-new" .)
}

run() {
  local work=$1 bin=$2 pass=$3
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

main() {
  set -euo pipefail
  local base=${1:-HEAD} repo work pass sub
  repo=$(git rev-parse --show-toplevel)
  work=$(mktemp -d)
  # shellcheck disable=SC2064 # expand now: work is local and gone by EXIT
  trap "trash '$work'" EXIT

  build "$repo" "$base" "$work"
  for sub in latest tag; do
    diff -u <("$work/his-base" "$sub" 400) <("$work/his-new" "$sub" 400)
    echo "==> $sub: equivalent, $("$work/his-base" "$sub" 400 | wc -l) lines by both"
  done
  for pass in empty copy; do
    run "$work" base "$pass"
    run "$work" new "$pass"
    diff -u "$work/$pass/base.log" "$work/$pass/new.log"
    diff -r "$work/$pass/base" "$work/$pass/new"
    echo "==> $pass: equivalent, $(wc -l <"$work/$pass/base.log") writes by both"
  done
}

main "$@"
