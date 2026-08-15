## Summary
<!-- One-line description of the change -->

## Checklist
- [ ] Updated the `## [Unreleased]` section in [CHANGELOG.md](CHANGELOG.md) (use Added / Changed / Fixed / Security etc. per Keep a Changelog)
- [ ] Tests pass locally: `cd gui-go && go test ./internal/crypto ./internal/vault ./internal/ui ./cmd/pulse-vault ./crypto -count=1`
- [ ] No real vault files, secrets, or large binaries committed
- [ ] Docs, metainfo, or packaging updated if user-facing or version change
- [ ] Cross-platform consideration (core + CLI paths should work on Windows/macOS)

## Related
<!-- Issue, roadmap item, or previous PR -->
