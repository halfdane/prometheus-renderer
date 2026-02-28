#!/usr/bin/env bash
set -e

cd "$(git rev-parse --show-toplevel)"

# Read current version from flake.nix
CURRENT=$(grep 'version = "' flake.nix | sed 's/.*version = "\([^"]*\)".*/\1/')
if [[ -z "$CURRENT" ]]; then
  echo "Could not determine current version from flake.nix" >&2
  exit 1
fi

# Auto-bump patch, or accept an explicit version as first argument
if [[ -n "$1" ]]; then
  NEXT="$1"
else
  IFS='.' read -r major minor patch <<< "$CURRENT"
  NEXT="$major.$minor.$((patch + 1))"
fi

echo "Bumping $CURRENT -> $NEXT"

# Patch flake.nix in place
sed -i "s/version = \"$CURRENT\"/version = \"$NEXT\"/" flake.nix

git add flake.nix
git commit -m "chore: bump version to v$NEXT"

TAG="v$NEXT"
git tag "$TAG"
git push origin main
git push origin "$TAG"

echo "Tagged $TAG"
