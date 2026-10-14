# Publish LazarusBinder

Everything for the first public GitHub release is prepared in the `LazarusBinder` folder: source, README, MIT licence, description, topics, release notes, issue templates, build automation, and Windows release assets.

## Quick publishing path

1. Install [Git for Windows](https://git-scm.com/download/win) and [GitHub CLI](https://cli.github.com/), then reopen PowerShell.
2. Open PowerShell inside the extracted `LazarusBinder` folder.
3. Sign in to your `kimineechan` GitHub account:

   ```powershell
   gh auth login --hostname github.com --git-protocol https --web --scopes workflow
   ```

4. Publish:

   ```powershell
   .\publish.cmd
   ```

The script creates the public `LazarusBinder` repository, pushes the code, sets its description and topics, and pushes `v1.0.0`. GitHub Actions then builds and tests the application and publishes the release ZIP and checksums. Open the Actions link printed by the script to see its progress.

No repository has been published by preparing this package. Running the publishing script is the step that creates the public GitHub repository.

## Website upload path

If you prefer GitHub's website, follow `LazarusBinder/docs/GITHUB.md`. That guide includes the exact description, topics, release title, source-upload instructions, and release assets to attach.

The prepared release ZIP is `LazarusBinder/dist/LazarusBinder-v1.0.0-windows-x64.zip`. It contains the same EXE you tested successfully. The source repository is the surrounding `LazarusBinder` folder; upload source files to the repository and put the Windows ZIP in Releases.

## Repository description

Portable Windows utility to bundle two standalone EXEs, set custom icons, and extract icons from EXE/DLL files.

## Topics

`windows` `golang` `desktop-app` `exe-binder` `exe-bundler` `executable` `portable-app` `icon-extractor` `ico` `win32` `windows-utility` `open-source`

## First release

- Tag: `v1.0.0`
- Title: `LazarusBinder v1.0.0`
- Notes: `LazarusBinder/docs/releases/v1.0.0.md`
- Asset: `LazarusBinder/dist/LazarusBinder-v1.0.0-windows-x64.zip`
- Checksums: `LazarusBinder/dist/SHA256SUMS.txt`
