#!/usr/bin/env bash

set -euo pipefail

expected=.github/workflows/run_tests.yml
mapfile -d '' active_workflows < <(
	find .github/workflows -maxdepth 1 -type f \
		\( -name '*.yml' -o -name '*.yaml' \) -print0 | sort -z
)

if (( ${#active_workflows[@]} != 1 )) || [[ ${active_workflows[0]:-} != "$expected" ]]; then
	printf 'Unexpected active GitHub workflow set:\n' >&2
	printf '  %s\n' "${active_workflows[@]:-(none)}" >&2
	printf 'Expected only:\n  %s\n' "$expected" >&2
	exit 1
fi
