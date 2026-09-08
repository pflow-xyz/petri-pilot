#!/usr/bin/env bash
# Shared-package lock between petri-pilot and sim-pflow-xyz.
#
# The two repos are different products that share four packages by copy:
# pkg/dsl, pkg/metamodel, pkg/validator, pkg/extensions (and, since the
# structural-readings port, pkg/prng and pkg/runtime/eventgen). Nothing
# reconciled them, and three real bug fixes sat on one side for weeks while
# the other kept the bugs. This is the same shape as the pflow-js.lock the
# JS consumers carry: a checksum lock the CI can fail on, not a network fetch.
#
#   scripts/shared-pkg.sh check            # every shared file matches the lock
#   scripts/shared-pkg.sh lock             # rewrite the lock from this tree
#   scripts/shared-pkg.sh diff  <sibling>  # show drift against the other checkout
#   scripts/shared-pkg.sh sync  <sibling>  # copy the shared files FROM the sibling
#
# The lock is identical in both repos on purpose: `check` passing in both
# means they carry the same bytes. When one side lands a fix, it runs `lock`,
# the other runs `sync <path-to-first>` and commits both the files and the
# lock. Test files are included, so a regression test travels with its fix.
set -euo pipefail
cd "$(dirname "$0")/.."
LOCK=shared-pkg.lock
SHARED=(pkg/dsl pkg/metamodel pkg/validator pkg/extensions pkg/prng pkg/runtime/eventgen)

files() {
  for d in "${SHARED[@]}"; do
    [ -d "$d" ] && find "$d" -maxdepth 1 -type f -name '*.go' | LC_ALL=C sort
  done
}
digest() { files | while read -r f; do printf '%s  %s\n' "$(sha256sum < "$f" | cut -c1-64)" "$f"; done; }

case "${1:-check}" in
  lock) digest > "$LOCK"; echo "wrote $LOCK ($(wc -l < "$LOCK") files)";;
  check)
    [ -f "$LOCK" ] || { echo "no $LOCK; run: $0 lock" >&2; exit 1; }
    if diff <(digest) "$LOCK" >/dev/null; then echo "shared packages match $LOCK"; else
      echo "shared packages drifted from $LOCK:" >&2; diff <(digest) "$LOCK" | grep '^[<>]' | awk '{print $NF}' | sort -u >&2
      echo "if this tree is the fix: $0 lock; if the other repo is: $0 sync <path>" >&2; exit 1; fi;;
  diff)
    sib=${2:?sibling checkout path}; rc=0
    for f in $(files); do [ -f "$sib/$f" ] || { echo "missing in sibling: $f"; rc=1; continue; }; cmp -s "$f" "$sib/$f" || { echo "differs: $f"; rc=1; }; done
    for d in "${SHARED[@]}"; do for f in $(cd "$sib" && [ -d "$d" ] && find "$d" -maxdepth 1 -type f -name '*.go'); do [ -f "$f" ] || { echo "missing here: $f"; rc=1; }; done; done
    [ $rc = 0 ] && echo "no drift against $sib"; exit $rc;;
  sync)
    sib=${2:?sibling checkout path}
    for d in "${SHARED[@]}"; do [ -d "$sib/$d" ] || continue; mkdir -p "$d"; cp "$sib/$d"/*.go "$d"/; done
    [ -f "$sib/$LOCK" ] && cp "$sib/$LOCK" "$LOCK"; echo "synced shared packages from $sib"; "$0" check;;
  *) echo "usage: $0 {check|lock|diff <sibling>|sync <sibling>}" >&2; exit 2;;
esac
