#!/usr/bin/env bash
set -euo pipefail

if [[ "$#" -eq 0 ]]; then
	echo "usage: scripts/ci-install-playwright.sh COMMAND [ARG...]" >&2
	exit 2
fi

# Prefer the public CDN over the platform-specific mirror. Bound retries so a
# persistent download failure still fails the job before browser tests start.
export PLAYWRIGHT_DOWNLOAD_HOST="${PLAYWRIGHT_DOWNLOAD_HOST:-https://cdn.playwright.dev}"

# Run one attempt. With a bound, a hung system-package mirror cannot hold the
# job for ten minutes per attempt.
run_attempt() {
	local bounded="$1"
	shift
	if [[ "${bounded}" == 1 ]] && command -v timeout > /dev/null 2>&1; then
		timeout --kill-after=15 "${CI_PLAYWRIGHT_DEPS_TIMEOUT:-240}" "$@"
	else
		"$@"
	fi
}

# Run up to three attempts with 5 s and 10 s backoff. Sets last_status.
# A timed-out bounded attempt ends the attempts at once.
last_status=0
run_attempts() {
	local bounded="$1"
	shift
	local attempt delay
	for attempt in 1 2 3; do
		if run_attempt "${bounded}" "$@"; then
			last_status=0
			return 0
		else
			last_status="$?"
		fi
		if [[ "${bounded}" == 1 && ("${last_status}" == 124 || "${last_status}" == 137) ]] && command -v timeout > /dev/null 2>&1; then
			echo "::warning::Playwright system-package install timed out (${last_status}); not retrying it" >&2
			return 1
		fi
		if [[ "${attempt}" -lt 3 ]]; then
			delay=$((attempt * 5))
			echo "::warning::Playwright install attempt ${attempt} failed (${last_status}); retrying in ${delay}s" >&2
			sleep "${delay}"
		fi
	done
	return 1
}

with_deps=0
for arg in "$@"; do
	if [[ "${arg}" == "--with-deps" ]]; then
		with_deps=1
	fi
done

if [[ "${with_deps}" == 0 ]]; then
	if run_attempts 0 "$@"; then
		exit 0
	fi
	exit "${last_status}"
fi

if run_attempts 1 "$@"; then
	exit 0
fi
status="${last_status}"

if [[ "${CI_PLAYWRIGHT_DEPS_FALLBACK:-1}" == 0 ]]; then
	exit "${status}"
fi

# System packages could not be installed. Try the browser alone and prove it
# launches with the libraries already on the runner image.
fallback=()
prefix=()
seen_install=0
for arg in "$@"; do
	if [[ "${arg}" == "install" ]]; then
		seen_install=1
	fi
	if [[ "${seen_install}" == 0 ]]; then
		prefix+=("${arg}")
	fi
	if [[ "${arg}" != "--with-deps" ]]; then
		fallback+=("${arg}")
	fi
done

echo "::warning::Playwright install with system packages failed (${status}); trying the browser without them" >&2
if ! run_attempts 0 "${fallback[@]}"; then
	echo "Playwright install without system packages failed (${last_status}); keeping status ${status}" >&2
	exit "${status}"
fi

probe_dir="$(mktemp -d)"
probe_status=0
"${prefix[@]}" screenshot --browser chromium about:blank "${probe_dir}/probe.png" || probe_status="$?"
rm -rf "${probe_dir}"
if [[ "${probe_status}" != 0 ]]; then
	echo "Chromium launch probe failed (${probe_status}); keeping status ${status}" >&2
	exit "${status}"
fi
echo "::warning::System packages could not be installed; Chromium launched with the runner image's preinstalled libraries" >&2
exit 0
