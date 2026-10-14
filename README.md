<p align="center"><img src="assets/logo.svg" width="96" height="96" alt="LazarusBinder icon"></p>

<h1 align="center">LazarusBinder</h1>
<p align="center">Two programs. One EXE. Your icon.</p>

<p align="center">
  <img src="https://img.shields.io/badge/platform-Windows%2010%20%2F%2011-0078D4" alt="Windows 10 and 11">
  <img src="https://img.shields.io/badge/language-Go-00ADD8" alt="Written in Go">
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-222222" alt="MIT licence"></a>
</p>

**LazarusBinder is a portable Windows utility for bundling two standalone EXEs into one file and managing executable icons.** Create a combined EXE, give it a custom ICO, or extract complete icon groups from an EXE or DLL.

[Download the latest Windows release](https://github.com/kimineechan/LazarusBinder/releases/latest) · [Usage guide](docs/USAGE.md) · [Report a bug](https://github.com/kimineechan/LazarusBinder/issues/new/choose)

## Features

| Feature | What it does |
| --- | --- |
| Bundle two programs | Package two standalone EXEs in a single combined EXE |
| Choose launch order | Start both together, or start program 2 after program 1 exits |
| Administrator support | Make the final combined EXE request administrator access through Windows UAC |
| Custom icons | Choose an ICO when bundling, or save an existing bundle with a new icon |
| Extract icons | Export individual or all icon groups from EXE/DLL resources, preserving their resolutions |
| Portable and local | Native Windows interface, no installer, network client, or telemetry |

The combined EXE starts the programs **without an extra launcher window**. It exits after startup while the applications continue running. In sequential mode, it waits in the background for the first process to finish before starting the second.

## Download and run

1. Open [Releases](https://github.com/kimineechan/LazarusBinder/releases).
2. Download `LazarusBinder-v1.0.0-windows-x64.zip` for the first release.
3. Extract the ZIP and open `LazarusBinder.exe`.

The builder targets **64-bit Windows 10 and Windows 11**. The bundled programs can be x86 or x64. The supplied executable needs no Go, Python, Node, or separate application framework.

Release assets include `SHA256SUMS.txt`. Check the ZIP in PowerShell with:

```powershell
Get-FileHash .\LazarusBinder-v1.0.0-windows-x64.zip -Algorithm SHA256
```

Compare the result with its entry in `SHA256SUMS.txt`.

## Create a combined EXE

1. Select **Program 1** and **Program 2**.
2. Enter a bundle name and optionally choose an `.ico` file.
3. Leave **Require administrator for the combined EXE** checked if either app needs administrator rights.
4. Optionally enable **Start program 2 after program 1 exits**.
5. Select **Create combined EXE** and save it under a new filename.

The administrator setting applies to the **final compiled EXE**. After Windows approves its UAC request, both programs inherit administrator rights. Cancelling that request prevents both from starting. The builder itself runs with normal permissions.

See the [usage guide](docs/USAGE.md) for the icon editor, icon extraction, cache locations, and launch behavior.

## Compatibility and limits

- The original input EXEs remain unchanged. The combined file contains verified copies and starts two separate Windows processes.
- Only the two selected EXE files are bundled. Nearby DLLs, configuration files, plugins, data folders, and installed runtimes are not included.
- Each input EXE may be up to 1 GiB.
- Extracted copies are stored under `%LOCALAPPDATA%\LazarusBinder\Runs\run-...`. Each launch creates a fresh folder; remove old folders after their applications have closed.
- Sequential launching waits for the first process, including nonzero exit codes. It does not follow detached processes that the first application creates.
- The icon editor updates LazarusBinder bundles by saving a new copy. Icon extraction can read arbitrary EXE/DLL resources.
- New combined EXEs are unsigned. The embedded input files retain their original bytes.
- To apply a launcher update to an older combined EXE, recreate the bundle from the two original inputs.

## Build from source

Install [Go](https://go.dev/dl/) 1.25 or newer, clone the repository, and open a terminal in its root:

```powershell
git clone https://github.com/kimineechan/LazarusBinder.git
cd LazarusBinder
.\build.cmd
```

The result is `dist/LazarusBinder.exe`. The Windows build script compiles both ordinary and administrator launcher templates, runs core and Win32 icon-resource tests, builds the editor, and verifies manifests and unchanged embedded program bytes.

The project uses the Go standard library and native Windows APIs. No third-party Go modules are required. The supplied `.syso` resource objects include the default icon and execution-level manifests; Python is needed only to regenerate those resource objects or create release ZIPs.

Create the release ZIP after building:

```powershell
python scripts/package_release.py
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for development checks and [docs/GITHUB.md](docs/GITHUB.md) for release publishing.

## How it works

The output consists of a Windows launcher PE, a ZIP overlay containing both programs and a JSON manifest, and a footer identifying the archive. The wrapper verifies the extracted files' sizes and SHA-256 hashes before starting either application.

The administrator template embeds `requireAdministrator`; the ordinary template and builder use `asInvoker`. The wrapper releases its handles to running children without terminating them. Icon extraction parses bounded PE resource data without executing the inspected program.

Implementation details and checks are in [VALIDATION.md](VALIDATION.md).

## Contributing and licence

Bug reports, reproducible compatibility reports, and focused pull requests are welcome. Please read [CONTRIBUTING.md](CONTRIBUTING.md).

MIT licence. See [LICENSE](LICENSE). The Go runtime and standard library notice is included in [THIRD-PARTY-NOTICES.txt](THIRD-PARTY-NOTICES.txt).

