# LazarusBinder usage guide

A portable Windows application that packages **two standalone EXEs into one combined EXE**, applies a custom icon to it, and extracts complete ICO files from EXE/DLL resources.

## Start here

1. Extract this ZIP to a folder on your PC.
2. Open `LazarusBinder.exe`.
3. Choose one of the three sections.

The app targets **64-bit Windows 10 and Windows 11**. You do not need Go, Node, Python or an installer to use the supplied EXE. Programs you bundle may still need their own external runtime installed.

## 01 — Bundle programs

- Select **Program 1** and **Program 2**: genuine x86 or x64 Windows EXE files.
- Leave **Require administrator for the combined EXE** checked. This setting applies to the final output EXE and is enabled by default.
- Enter a name for the bundle's metadata.
- Optionally choose an `.ico` file. A small preview appears beside it.
- Leave **Start program 2 after program 1 exits** unchecked to launch both together. Check it for sequential launching.
- Click **Create combined EXE** and choose a new filename, such as `Combined.exe`.

Double-click the resulting EXE. It verifies and extracts both programs, then starts them automatically **without an extra launcher window**. When both have started, the combined EXE exits and the programs continue running. Only the programs' own windows and any required Windows administrator prompt appear during successful startup. They remain two separate Windows processes; their machine code is not merged. Startup failures still show an error dialog.

Each program is extracted into its own folder under `%LOCALAPPDATA%\LazarusBinder\Runs\run-...`, preserving its original filename. Its working directory is that folder. Open that path in Explorer to inspect the extracted copies. The original EXEs stay unchanged.

In sequential mode, the combined EXE waits in the background for the first process to exit, starts program 2, then exits. It does not track any detached child processes that application creates.

**Updating an older combined EXE:** create a new bundle from the original two EXEs using this updated LazarusBinder. Existing bundles keep their original launcher code; changing only their icon does not apply this startup update.

### Administrator access on the final combined EXE

With **Require administrator for the combined EXE** checked, the final `Combined.exe` contains a Windows manifest with `requestedExecutionLevel="requireAdministrator"`. Open that file normally. Windows handles the administrator consent or credentials before the launcher starts. After approval, it launches both embedded programs from the elevated process, so both inherit administrator rights. This allows the embedded app that requires administrator access to start with those rights; the other app also runs elevated.

The original two EXEs are unchanged. The administrator requirement belongs to the generated combined EXE. The LazarusBinder builder itself uses `asInvoker` and does not need elevation to create a bundle in a writable folder.

Cancelling the combined EXE's elevation request prevents the launcher and both embedded programs from starting. Windows' UAC configuration determines whether a prompt appears. Uncheck the setting to generate an ordinary `asInvoker` launcher instead.

Rebuild an existing bundle to add this embedded administrator requirement. You can also run an existing combined EXE using Windows' **Run as administrator** menu option, which gives its child processes the elevated launcher's permissions. The icon editor preserves the execution level of the bundle it copies.

## 02 — Set bundle icon

1. Choose an EXE previously created by LazarusBinder.
2. Choose a new `.ico` file.
3. Click **Save EXE with new icon** and choose a different output filename.

This section saves a new bundle copy. It changes the launcher's Windows icon and keeps both embedded EXEs intact. It supports LazarusBinder bundles, including bundles copied to another computer. It does not edit arbitrary third-party EXEs.

If Explorer still shows a cached icon, try the new output filename or refresh its folder. Multi-resolution ICOs generally display more cleanly than a single small image.

## 03 — Extract icons

1. Click **Load icons** and select a Windows `.exe` or `.dll` file.
2. Select an icon group from the list to preview it and see its sizes.
3. Choose **Export selected ICO** or **Export all groups**.

Each exported ICO preserves the group's original image data and resolutions. **Export all groups** asks for a base filename and creates a new sibling folder ending in `-groups`; it does not overwrite an existing folder. Files without embedded icon groups produce an explanatory message. Selecting or inspecting a file never runs it.

You can reuse an exported ICO in either of the other sections. This version accepts ICO files directly; PNG/JPG conversion is not included.

## Supported features

| Feature | Behaviour |
| --- | --- |
| Combined file | One self-contained launcher containing two EXE files |
| Startup | Both together, or program 2 after program 1 exits |
| Administrator access | Final combined EXE requests administrator rights; both apps inherit them |
| Visibility | No extra launcher window; wrapper exits after startup |
| Custom icon | At bundle creation or later on a new bundle copy |
| Icon extraction | EXE/DLL icon groups, preserving embedded resolutions |
| Maximum input | 1 GiB per EXE |
| Architecture | x64 launcher; x86 and x64 bundled programs |
| Storage | Local files only; the app has no network client or telemetry |

**Use standalone applications that you trust.** This version embeds the two chosen files only. Programs that require neighbouring DLLs, plugins, configuration files, data folders, or an installation may fail because those supporting files are not included. Bundling an installer runs that installer; it does not turn it into an installed portable application. For a program requiring administrator rights, leave **Require administrator for the combined EXE** checked. Cancelling the final EXE's elevation request prevents both apps from starting.

Extracted files remain in the Runs folder, allowing applications to retain files they create beside their EXE. After closing all launched applications, you can delete old `run-...` folders there. Each launch creates a fresh folder, so repeated launches use additional disk space and do not reuse a previous run's settings.

The new launcher is unsigned. Original embedded program bytes are preserved, including any signatures they already contain. SHA-256 and ZIP checks detect accidental corruption; an unsigned bundle is not a proof of who supplied its contents.

For source builds and development, see [the README](../README.md).
