#!/usr/bin/env sh
set -eu

version="${1:-}"

case "$version" in
  v[0-9]*.[0-9]*.[0-9]*)
    ;;
  *)
    echo "Usage: $0 v0.1.1" >&2
    exit 1
    ;;
esac

case "$version" in
  *[!A-Za-z0-9._-]*)
    echo "Invalid version: $version" >&2
    exit 1
    ;;
esac

if [ ! -f index.php ] || [ ! -f README.md ]; then
  echo "Run this script from the Dockan-Panel repository root." >&2
  exit 1
fi

tmp="$(mktemp)"
sed "s/^const APP_VERSION = '.*';$/const APP_VERSION = '${version}';/" index.php > "$tmp"
mv "$tmp" index.php

tmp="$(mktemp)"
sed "s/^Current panel version: \`.*\`\.$/Current panel version: \`${version}\`./" README.md > "$tmp"
mv "$tmp" README.md

echo "Dockan Panel version set to ${version}"
