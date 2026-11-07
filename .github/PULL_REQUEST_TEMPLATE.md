## Summary
<!-- One-line description of the change -->

## Checklist
- [ ] Updated the `## [Unreleased]` section in [CHANGELOG.md](CHANGELOG.md) (use Added / Changed / Fixed / Security etc. per Keep a Changelog)
- [ ] Headless Go tests pass from the repository root: `CGO_ENABLED=0 go test ./... -count=1` (PowerShell: `$env:CGO_ENABLED='0'; go test ./... -count=1`)
- [ ] No real vault files, secrets, or large binaries committed
- [ ] Docs, metainfo, or packaging updated if user-facing or version change
- [ ] Cross-platform consideration (core + CLI paths should work on Windows/macOS)

## Related
<!-- Issue, roadmap item, or previous PR -->
