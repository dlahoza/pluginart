# Releasing

Release tags use `v<version>`, for example `v0.2.0`. Python and npm package versions should match the tag without the `v`.

Checklist:

1. Update Go, Python, and npm versions.
2. Run Go tests, Python package tests, TypeScript build/tests, `python -m build`, and `npm pack --dry-run`.
3. Tag the release: `git tag v0.2.0 && git push origin v0.2.0`.
4. Confirm PyPI trusted publishing is configured for `.github/workflows/release.yml`.
5. Confirm npm trusted publishing or `NPM_TOKEN` is configured for package `pluginart`.
