#!/usr/bin/env bash
# Same grammar as CI (commitlint @commitlint/config-conventional):
# types, lowercase subject, no trailing period, header and body lines <= 100.
set -euo pipefail

file=${1:-}
if [[ -n $file ]]; then
	raw=$(cat "$file")
else
	raw=$(cat)
fi

types='feat|fix|docs|style|refactor|perf|test|build|ci|chore|revert'
header_re="^(${types})(\\([a-z0-9._/-]+\\))?(!)?: [^[:space:]].*$"

strip() {
	# Drop git comment lines and trailing whitespace on each line.
	printf '%s\n' "$raw" | sed -e '/^#/d' -e 's/[[:space:]]*$//'
}

body=$(strip)
# Trim leading/trailing blank lines.
while [[ $body == $'\n'* ]]; do
	body=${body#$'\n'}
done
while [[ $body == *$'\n' ]]; do
	body=${body%$'\n'}
done

if [[ -z $body ]]; then
	echo "commit-msg: empty message" >&2
	exit 1
fi

# Merge / revert / fixup commits are ignored by commitlint's defaultIgnores.
first=${body%%$'\n'*}
case $first in
Merge\ * | Revert\ * | fixup!\ * | squash!\ *)
	exit 0
	;;
esac

if [[ ${#first} -gt 100 ]]; then
	echo "commit-msg: header is ${#first} characters (max 100)" >&2
	exit 1
fi

if ! [[ $first =~ $header_re ]]; then
	echo "commit-msg: subject must be type(scope)?: lowercase summary" >&2
	echo "  got: $first" >&2
	exit 1
fi

if [[ $first == *. ]]; then
	echo "commit-msg: subject must not end with a period" >&2
	exit 1
fi

# Subject text after ": " must not start with uppercase (commitlint subject-case).
subject=${first#*: }
if [[ $subject == [[:upper:]]* ]]; then
	echo "commit-msg: subject must start with lowercase" >&2
	exit 1
fi

if [[ $body == *$'\n'* ]]; then
	rest=${body#*$'\n'}
	second=${rest%%$'\n'*}
	if [[ -n $second ]]; then
		echo "commit-msg: blank line required after the subject" >&2
		exit 1
	fi
	while IFS= read -r line; do
		if [[ ${#line} -gt 100 ]]; then
			echo "commit-msg: body line is ${#line} characters (max 100)" >&2
			echo "  $line" >&2
			exit 1
		fi
	done <<<"$rest"
fi
