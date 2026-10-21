#!/usr/bin/env bash
set -euo pipefail

if [[ "$#" -eq 0 ]]; then
	echo "usage: scripts/ci-install-playwright.sh COMMAND [ARG...]" >&2
	exit 2
fi

# Prefer the public CDN over the platform-specific mirror. Bound retries so a
# persistent download failure still fails the job before browser tests start.
export PLAYWRIGHT_DOWNLOAD_HOST="${PLAYWRIGHT_DOWNLOAD_HOST:-https://cdn.playwright.dev}"
status=0
for attempt in 1 2 3; do
	if "$@"; then
		exit 0
	else
		status="$?"
	fi
	if [[ "${attempt}" -lt 3 ]]; then
		delay=$((attempt * 5))
		echo "::warning::Playwright install attempt ${attempt} failed (${status}); retrying in ${delay}s" >&2
		sleep "${delay}"
	fi
done
exit "${status}"
