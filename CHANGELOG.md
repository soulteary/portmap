# Changelog

All notable changes to portmap are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project follows [Semantic Versioning](https://semver.org/).

## [1.3.3] - 2026-10-28

### Changed

- Update golang.org/x/net from 0.61.0 to 0.62.0; keep Go 1.27.3.
- Align CI report binaries with their pinned Action wrappers: go-test-report
  1.1.3 and goreportcard 1.1.4.
- Refresh artifact upload/download, container login and GoReleaser Actions.

## [1.3.2] - 2026-10-22

- Require Go 1.27.3 and align CI, documentation and report toolchain restoration.
- Include the compatible dependency updates recorded in the
  [published release](https://github.com/soulteary/portmap/releases/tag/v1.3.2).

## [1.3.1] - 2026-10-11

- Build release artifacts with Go 1.27.2 and align CI and packaging tools.
- Include the merged dependency and CI maintenance updates recorded in the
  [published release](https://github.com/soulteary/portmap/releases/tag/v1.3.1).

## [1.3.0] - 2026-09-06

- Bound SSH agent operations by dial timeout and default forward session resources.
- Run the container as a non-root user.
- Reject conflicting multi-instance admin options, HTTP Upgrade requests and
  empty selected service sections.
- Handle Expect: 100-continue without deadlock and remove the macOS timing
  dependency from the connection-counter test.
- These entries summarize the
  [published release](https://github.com/soulteary/portmap/releases/tag/v1.3.0).

## [1.2.0] - 2026-09-05

### Added

- Added a single-port SOCKS5 and HTTP/HTTPS proxy with direct, SOCKS5, HTTP
  CONNECT, and SSH upstream dialing.
- Added YAML multi-instance configuration for forwarders and proxies, including
  validation for duplicate listeners and conflicting shared admin endpoints.
- Added optional JSON/Prometheus statistics endpoints and a browser-based
  monitoring panel with bounded connection-event history.
- Added the self-contained `cmd/loadtest` benchmark tool with TCP/UDP modes,
  bounded latency sampling, JSON output, CI thresholds, and exact latency
  extrema.
- Extended the existing six-language CLI, log, and error localization to the new
  commands and added contributor guidance, Dependabot updates, and a locally
  generated Go Report Card report.

### Changed

- Raised the build requirement to Go 1.27.
- Limited supported release targets to Linux and macOS on amd64 and arm64.
  Windows binaries are no longer built or supported.
- Hardened the release workflow with pinned actions, module/vet/race/security
  gates, Docker Hub credential validation, checksums, and Sigstore build
  attestations.
- Made TCP idle timeout apply to the shared tunnel rather than independently
  expiring one direction while traffic is still flowing in the other.

### Fixed

- Reject forward targets that resolve back to the active listener, including
  wildcard, dual-stack, omitted-host, unspecified-address, service-port, DNS
  rebinding, and scoped IPv6-zone cases.
- Bound proxy protocol detection and request parsing independently from outbound
  dialing, and reject HTTP request lines and headers larger than 1 MiB.
- Hardened SSH upstream lifecycle handling, keepalive probes, reconnects,
  shutdown cancellation, encrypted-key passphrases, and host-key validation.
- Kept connection statistics and Web event IDs consistent across open, close,
  reject, and dial-error paths.
- Preserved explicit multi-instance settings and rejected invalid combinations
  before starting partial service.

## [1.1.0] - 2026-08-08

- See the [v1.1.0 release](https://github.com/soulteary/portmap/releases/tag/v1.1.0)
  and the comparison below for the complete change history.

## [1.0.0] - 2026-07-14

- Initial stable release.

[1.2.0]: https://github.com/soulteary/portmap/compare/v1.1.0...v1.2.0
[1.1.0]: https://github.com/soulteary/portmap/compare/v1.0.0...v1.1.0
[1.0.0]: https://github.com/soulteary/portmap/releases/tag/v1.0.0

[1.3.3]: https://github.com/soulteary/portmap/compare/v1.3.2...v1.3.3
[1.3.2]: https://github.com/soulteary/portmap/compare/v1.3.1...v1.3.2
[1.3.1]: https://github.com/soulteary/portmap/compare/v1.3.0...v1.3.1
[1.3.0]: https://github.com/soulteary/portmap/compare/v1.2.0...v1.3.0
