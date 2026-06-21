#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 1 || $# -gt 2 ]]; then
	echo "usage: scripts/release-notes.sh <version> [release-notes-file]" >&2
	exit 2
fi

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
version="$1"
notes_file="${2:-${ROOT}/docs/RELEASE_NOTES.md}"

if [[ ! -f "${notes_file}" ]]; then
	echo "release notes file not found: ${notes_file}" >&2
	exit 1
fi

notes="$(
	awk -v version="${version}" '
		$0 == "## " version || index($0, "## " version " - ") == 1 {
			in_section = 1
			seen = 1
			next
		}
		in_section && /^## / {
			exit
		}
		in_section {
			lines[++n] = $0
		}
		END {
			if (!seen) {
				printf("release notes section not found for %s\n", version) > "/dev/stderr"
				exit 1
			}
			start = 1
			while (start <= n && lines[start] == "") start++
			end = n
			while (end >= start && lines[end] == "") end--
			if (start > end) {
				printf("release notes section is empty for %s\n", version) > "/dev/stderr"
				exit 1
			}
			for (i = start; i <= end; i++) print lines[i]
		}
	' "${notes_file}"
)"

if [[ "${notes}" == *\\n* ]]; then
	echo "release notes for ${version} contain a literal \\n sequence" >&2
	exit 1
fi

printf '%s\n' "${notes}"
