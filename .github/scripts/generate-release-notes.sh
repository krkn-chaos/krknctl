#!/usr/bin/env bash

set -euo pipefail

: "${VERSION:?VERSION must be set}"
OUTPUT=${OUTPUT:-release-notes.md}
TEMPLATE=${TEMPLATE:-.github/release-template.md}

previous_tag=$(git describe --first-parent --tags --match 'v*' --exclude "${VERSION}" --abbrev=0 "${VERSION}" 2>/dev/null || true)

if [[ -n "${previous_tag}" ]]; then
	changes=$(git log --no-merges --pretty=format:'- %s (%h)' "${previous_tag}..${VERSION}")
else
	changes=$(git log --no-merges --pretty=format:'- %s (%h)' "${VERSION}")
fi

if [[ -z "${changes}" ]]; then
	changes='- No changes recorded.'
fi

VERSION="${VERSION}" CHANGES="${changes}" perl -0pe \
	's/\{VERSION\}/$ENV{VERSION}/g; s/\{CHANGES\}/$ENV{CHANGES}/g' \
	"${TEMPLATE}" > "${OUTPUT}"
