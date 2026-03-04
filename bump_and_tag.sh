#!/usr/bin/env bash
set -e

cd "$(git rev-parse --show-toplevel)"

# ── Pre-flight checks ───────────────────────────────────────────────────
echo "Running tests..."
go test ./...

echo "Running vet..."
go vet ./...

echo "Running staticcheck..."
staticcheck ./...

# If checks changed any tracked files, abort.
if ! git diff --quiet HEAD; then
  echo "⚠️  Working tree has uncommitted changes." >&2
  echo "   Please review, commit, and re-run." >&2
  exit 1
fi

# ── Bump version ─────────────────────────────────────────────────────────
CURRENT=$(grep 'renderVersion = "' flake.nix | sed 's/.*renderVersion = "\([^"]*\)".*/\1/')
if [[ -z "$CURRENT" ]]; then
  echo "Could not determine current version from flake.nix" >&2
  exit 1
fi

if [[ -n "$1" ]]; then
  NEXT="$1"
else
  IFS='.' read -r major minor patch <<< "$CURRENT"
  NEXT="$major.$minor.$((patch + 1))"
fi

echo "Bumping $CURRENT -> $NEXT"

sed -i "s/renderVersion = \"$CURRENT\"/renderVersion = \"$NEXT\"/" flake.nix

git add flake.nix
git commit -m "chore: bump version to v$NEXT"

TAG="v$NEXT"
git tag "$TAG"
git push origin main
git push origin "$TAG"

echo "Tagged $TAG"
