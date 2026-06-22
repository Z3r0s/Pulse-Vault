# Pulse-Vault Roadmap

Official site: [dnspulse.org](https://dnspulse.org)

Source: [github.com/Z3r0s/Pulse-Vault](https://github.com/Z3r0s/Pulse-Vault)

## Shipped: 0.2.0

- dnspulse.org wired as Homepage in package metadata
- Window icon, empty state, light/dark file table theming
- Determinate progress for add, extract, and verify
- Search debounce, double-click extracts instead of Secure Open

## 0.3 — Distribution

- Publish to PyPI (`pip install pulse-vault`)
- Document system dependency: `python3-tk` on Debian/Ubuntu
- Optional `[gui]` extra in pyproject if a headless path appears later
- TestPyPI dry run and clean-venv install smoke test on Linux + Windows

## 0.4 — Trust assets

- Three screenshots for README, GitHub, and AppStream
- Short demo GIF (create vault, drag file, lock, reopen)
- AppStream `<screenshots>` block

## 0.5 — GUI overhaul

Make the whole app feel like finished DNSPulse desktop software, not a strong prototype.

- Sidebar logo mark from `pulse-vault.png` instead of text-only branding
- File-type icons or suffix badges in the file table
- Keyboard shortcuts (lock, add, extract, search focus)
- Responsive layout for smaller laptop widths
- First-run welcome panel with clear primary actions
- Context menu and dialogs follow appearance mode consistently
- Inline status toasts for routine success; reserve modal dialogs for risky actions
- Sortable columns in the file table
- Recent vaults list in the sidebar
- Settings panel: default extract folder, confirm-before-delete toggle
- Product page on [dnspulse.org](https://dnspulse.org) matching app visuals (colors, typography, icon)

## 0.6 — Performance

- Incremental ZIP updates so adding one file does not copy every existing `data/*.enc` blob
- Metadata-only quick verify option for large vaults
- Trim the 5 MB header probe on unlock for normal ZIP vaults

## 1.0 — Growth

- Flathub manifest
- Optional CLI: `pulse-vault add`, `extract`, `verify`
- Carrier-file round-trip tests and docs
- Launch post (Show HN, r/linux, r/privacy) with honest threat model link

## Manual smoke checklist (before each release)

- [ ] Create vault, add file via dialog and drag-drop
- [ ] Extract and verify vault integrity
- [ ] Rotate password on a vault with multiple files
- [ ] Toggle light/dark appearance and confirm table readability
- [ ] Open legacy `.PulseVault` and confirm rename prompt
- [ ] Lock vault and confirm empty state returns