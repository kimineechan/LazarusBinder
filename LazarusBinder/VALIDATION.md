# Validation

## Checks completed on 14 October 2026

The packaged Windows EXE was compiled with Go 1.27.2 from Linux for `windows/amd64`, with CGO disabled.

- Core tests and the Go race detector passed for bundle creation/extraction, input-byte preservation, invalid input handling, archive bounds, checksum failures, ICO parsing, and launch coordination.
- Concurrent launch tests confirm that the wrapper returns while both programs remain running and releases both process handles. One failed startup or pending administrator request does not prevent the other program starting.
- Sequential launch tests confirm that the wrapper waits for program 1 and releases program 2 without waiting for it. Program 2 is skipped if the first cannot start or be tracked.
- Release verification checked the actual manifests and default icons in the builder and both launcher templates. Both administrator and ordinary output bundles preserve the two input EXEs byte for byte.
- Administrator output contains `requireAdministrator`; ordinary output and the builder contain `asInvoker`.
- Windows-targeted `go vet` passed. Win32 icon-resource and bundle-retouch tests compiled successfully.
- All three executables use the AMD64 architecture and Windows GUI subsystem.
- The old launch-status window was removed from both output templates.
- The maintainer subsequently reported that the updated program runs successfully on Windows.

The maintainer's report confirms their tested launch scenario. It is not a test matrix for every Windows version, application, UAC configuration, or icon operation.

## GitHub checks

The included Windows workflow builds both launcher modes, runs native Win32 resource tests, verifies the generated EXEs, and packages a ZIP with SHA-256 checksums. A Linux job separately runs the core tests with the race detector. A version tag publishes the release only after both jobs succeed.

These workflows are prepared for the repository; their first remote result will be available in GitHub Actions after publication. The included source and prebuilt EXE do not claim an already-completed GitHub run.

## Windows acceptance checks

Use two small standalone applications to check:

1. Concurrent launch: no extra wrapper window; the combined EXE exits while the applications remain open.
2. Sequential launch: program 2 starts after the first process exits, then the combined EXE exits.
3. Administrator mode: both apps run elevated after approval; neither starts when the combined EXE's UAC request is cancelled.
4. Ordinary mode: both apps run with normal permissions when opened normally.
5. Icon editing: a new bundle copy has the selected ICO and preserves both input files and the administrator setting.
6. Icon extraction: exported groups preserve their original resolutions.

A native Windows desktop was not available in the build environment. Interactive dialogs, UAC behavior, and compatibility beyond the maintainer's reported scenario still need native Windows verification.

