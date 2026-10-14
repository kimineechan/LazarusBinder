# Contributing

Thanks for helping improve LazarusBinder.

## Report a problem

Use the bug-report form and include the Windows version, LazarusBinder version, launch mode, administrator setting, and steps to reproduce. Quote any error text. A small public standalone test application is useful when a problem depends on the input program.

For feature requests, describe the workflow and the result you want.

## Develop

Install Go 1.25 or newer. The repository has no third-party Go modules.

Core tests on any supported development platform:

```sh
go test ./internal/...
```

Format Go changes with `gofmt`. On Windows, run `build.cmd` before submitting a change. It generates both embedded launcher templates, exercises Win32 resource updates, builds the editor, runs `go vet`, and verifies the generated files.

The generated files `cmd/lazarusbinder/launcher.exe` and `launcher-admin.exe` are build inputs and are not committed. Build them through the script before testing or compiling the Windows editor directly.

To regenerate icon/manifest resources after changing their source assets:

```sh
python scripts/make_resources.py assets/lazarusbinder.ico assets/app.manifest assets/launcher-normal.syso
python scripts/make_resources.py assets/lazarusbinder.ico assets/launcher-admin.manifest assets/launcher-admin.syso
python scripts/make_resources.py assets/lazarusbinder.ico assets/app.manifest cmd/lazarusbinder/resources_windows_amd64.syso
```

Keep `cmd/launcher/resources_windows_amd64.syso` equal to the ordinary launcher resource object between builds.

## Pull requests

Keep a pull request focused. Explain the problem, resulting behavior, and checks performed. Add tests when a change affects parsing, integrity, process handling, or another behavior that can regress.

Preserve the bundle footer format and legacy metadata compatibility when changing bundle code. Applications should retain their own visible behavior and Windows should continue to handle UAC normally.

## Releases

Update `VERSION`, `CHANGELOG.md`, and `docs/releases/v<version>.md` together. Push the matching `v<version>` tag. The release workflow builds on Windows, runs tests, packages the executable, and publishes the ZIP and checksums after the Linux and Windows jobs pass.

See [the publishing guide](docs/GITHUB.md).

