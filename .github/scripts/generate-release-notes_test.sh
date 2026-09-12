#!/usr/bin/env bash

set -euo pipefail

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
work_dir=$(mktemp -d)
trap 'rm -rf "${work_dir}"' EXIT

git_config() {
	git -C "${work_dir}" config user.email test@example.com
	git -C "${work_dir}" config user.name test
	git -C "${work_dir}" config commit.gpgsign false
}

commit() {
	local message=$1
	echo "${message}" >> "${work_dir}/history.txt"
	git -C "${work_dir}" add history.txt
	git -C "${work_dir}" commit -m "${message}" >/dev/null
}

assert_contains() {
	local value=$1
	local expected=$2
	if [[ "${value}" != *"${expected}"* ]]; then
		echo "expected output to contain: ${expected}" >&2
		exit 1
	fi
}

git -C "${work_dir}" init -q
git_config
commit "first release"
git -C "${work_dir}" tag v1.0.0
commit "feature after release"
git -C "${work_dir}" tag internal-checkpoint
commit "fix after non-release tag"
git -C "${work_dir}" tag v1.1.0

cat > "${work_dir}/template.md" <<'EOF'
# {VERSION}

{CHANGES}
EOF

(cd "${work_dir}" && VERSION=v1.1.0 OUTPUT="${work_dir}/notes.md" TEMPLATE="${work_dir}/template.md" \
	bash "${script_dir}/generate-release-notes.sh")
notes=$(<"${work_dir}/notes.md")
assert_contains "${notes}" '# v1.1.0'
assert_contains "${notes}" 'feature after release'
assert_contains "${notes}" 'fix after non-release tag'

git -C "${work_dir}" tag v1.2.0
(cd "${work_dir}" && VERSION=v1.2.0 OUTPUT="${work_dir}/empty.md" TEMPLATE="${work_dir}/template.md" \
	bash "${script_dir}/generate-release-notes.sh")
assert_contains "$(<"${work_dir}/empty.md")" '- No changes recorded.'

first_dir=$(mktemp -d)
trap 'rm -rf "${work_dir}" "${first_dir}"' EXIT
git -C "${first_dir}" init -q
git -C "${first_dir}" config user.email test@example.com
git -C "${first_dir}" config user.name test
git -C "${first_dir}" config commit.gpgsign false
echo first > "${first_dir}/history.txt"
git -C "${first_dir}" add history.txt
git -C "${first_dir}" commit -m 'first-ever release' >/dev/null
git -C "${first_dir}" tag v0.1.0
(cd "${first_dir}" && VERSION=v0.1.0 OUTPUT="${first_dir}/notes.md" TEMPLATE="${work_dir}/template.md" \
	bash "${script_dir}/generate-release-notes.sh")
assert_contains "$(<"${first_dir}/notes.md")" 'first-ever release'
