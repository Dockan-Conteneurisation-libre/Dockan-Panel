#!/usr/bin/env sh
# Sets the panel version in every file that shows it.
set -eu

version="${1:-}"
case "$version" in
  v[0-9]*.[0-9]*.[0-9]*) ;;
  *) echo "Usage: $0 v0.2.1" >&2; exit 1 ;;
esac
case "$version" in
  *[!A-Za-z0-9._-]*) echo "Invalid version: $version" >&2; exit 1 ;;
esac
if [ ! -f internal/config/config.go ] || [ ! -f web/package.json ]; then
  echo "Run this script from the Dockan-Panel repository root." >&2
  exit 1
fi

bare="${version#v}"
replace() {
  tmp="$(mktemp)"
  sed "$1" "$2" > "$tmp"
  cat "$tmp" > "$2"
  rm -f "$tmp"
}

replace "s/^var Version = \".*\"$/var Version = \"${version}\"/" internal/config/config.go
replace "s/^  \"version\": \".*\",$/  \"version\": \"${bare}\",/" web/package.json
for readme in README.md docs/i18n/README.*.md; do
  replace "s/version-v[0-9][0-9A-Za-z.-]*-/version-${version}-/; s/\`v[0-9][0-9.]*\`\(\.\{0,1\}\)\$/\`${version}\`\1/" "$readme"
done
replace "s/>v[0-9][0-9.]*</>${version}</" docs/assets/badges/version.svg

echo "Dockan Panel version set to ${version}"
