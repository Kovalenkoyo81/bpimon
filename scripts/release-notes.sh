#!/usr/bin/env bash
# Prints the CHANGELOG.md section for a version, e.g.:
#   scripts/release-notes.sh v1.7.1
# Exits with an error if the section is missing or empty, so a release is
# never published without notes.
set -euo pipefail

version=${1:?usage: $0 vX.Y.Z}
version=${version#v}
changelog=${CHANGELOG:-CHANGELOG.md}

notes=$(awk -v v="$version" '
  /^## \[/ {
    if (found) exit
    if (index($0, "## [" v "]") == 1) { found = 1; next }
  }
  /^\[[^]]+\]: / { if (found) exit }
  found { print }
' "$changelog" | sed -e '/./,$!d' | sed -e :a -e '/^\n*$/{$d;N;ba' -e '}')

if [ -z "$notes" ]; then
  echo "error: $changelog has no section for $version — add \"## [$version] - YYYY-MM-DD\"" >&2
  exit 1
fi
printf '%s\n' "$notes"
