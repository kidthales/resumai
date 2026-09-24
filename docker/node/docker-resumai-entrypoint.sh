#!/bin/sh
set -e

if [ "$1" = 'node' ] || [ "$1" = 'npx' ]; then
	if [ -z "$(ls -A 'node_modules/' 2>/dev/null)" ]; then
		npm ci
	fi
fi

exec docker-entrypoint.sh "$@"
