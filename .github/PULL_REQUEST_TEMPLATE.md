## Summary
<!-- One-line description of the change -->

## Checklist
- [ ] Updated the `## [Unreleased]` section in [CHANGELOG.md](CHANGELOG.md) (use Added / Changed / Fixed / Security etc. per Keep a Changelog)
- [ ] Tests pass locally: `PULSEVAULT_TEST_FAST_KDF=1 python -m unittest discover -s tests -v`
- [ ] No real vault files, secrets, or large binaries committed
- [ ] Docs, metainfo, or packaging updated if user-facing or version change
- [ ] Cross-platform consideration (core + CLI paths should work on Windows/macOS; GUI is mocked for CI)
- [ ] Advanced tests / Hypothesis properties still make sense if you touched crypto or vault I/O

## Related
<!-- Issue, roadmap item, or previous PR -->
