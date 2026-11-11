## Summary
<!-- One-line description of the change -->

## Screenshots
<!-- For GUI or documentation changes, attach the normal view and a compact-window view. -->

## Validation
- [ ] Headless Go tests pass from the repository root: `CGO_ENABLED=0 go test ./... -count=1` (PowerShell: `$env:CGO_ENABLED='0'; go test ./... -count=1`)
- [ ] `go vet ./...` passes
- [ ] Changed behavior has regression coverage
- [ ] GUI changes reviewed at 1120×740 and 640×480; screenshots attached
- [ ] Performance claims include a reproducible workload, method, and measured results
- [ ] Benchmark output contains no host, OS, hardware, architecture, or account identifiers
- [ ] Protocol or container changes update the format docs and compatibility/tamper tests

## Checklist
- [ ] Updated the `## [Unreleased]` section in [CHANGELOG.md](../CHANGELOG.md) (use Added / Changed / Fixed / Security etc. per Keep a Changelog)
- [ ] No real vault files, secrets, or large binaries committed
- [ ] Docs, metainfo, or packaging updated if user-facing or version change
- [ ] Cross-platform consideration (core + CLI paths should work on Windows/macOS)

## Related
<!-- Issue, roadmap item, or previous PR -->
