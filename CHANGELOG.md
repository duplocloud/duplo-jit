## 2026-02-24

### Added
- Opt-in auth cooldown to prevent browser tab spam when multiple processes request interactive credentials simultaneously. Set `DUPLO_JIT_AUTH_COOLDOWN=true` (or a duration like `30m`) to enable. Thanks to @scholzie for the original contribution in #52.

### Changed
- Upgraded all direct and indirect Go module dependencies.

## 2024-02-14

### Added
- Introduced a new section in the README for Homebrew installation, enhancing the accessibility of the tool for macOS users.

## 2024-01-24

### Fixed
- Improved error message format when a tenant is missing or not allowed.
- Prevented appending a nil error object to fatal error messages.