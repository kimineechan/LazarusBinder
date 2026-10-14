# Publish LazarusBinder on GitHub

## Repository settings

| Setting | Value |
| --- | --- |
| Owner | `kimineechan` |
| Name | `LazarusBinder` |
| Visibility | Public |
| Default branch | `main` |
| Licence | MIT |
| First release tag | `v1.0.0` |
| First release title | `LazarusBinder v1.0.0` |

**Description**

```text
Portable Windows utility to bundle two standalone EXEs, set custom icons, and extract icons from EXE/DLL files.
```

**Topics**

```text
windows, golang, desktop-app, exe-binder, exe-bundler, executable, portable-app, icon-extractor, ico, win32, windows-utility, open-source
```

These settings are also stored in `.github/repository.json`. GitHub does not apply that JSON automatically; the publishing script reads and applies it.

## Recommended: publish from Windows

Install [Git for Windows](https://git-scm.com/download/win) and [GitHub CLI](https://cli.github.com/). Reopen PowerShell after installation.

In the extracted `LazarusBinder` folder:

```powershell
gh auth login --hostname github.com --git-protocol https --web --scopes workflow
.\publish.cmd
```

Use the GitHub account `kimineechan` when signing in. Authentication is handled by GitHub CLI and your browser.

The script:

1. Initializes a Git repository in this project folder and commits the source and documentation.
2. Creates the public `kimineechan/LazarusBinder` repository and pushes `main`.
3. Adds the description and topics, enables issues, and sets the default branch.
4. Creates and pushes the annotated `v1.0.0` tag.
5. Prints the repository, Actions, and Releases links.

The GitHub Actions workflow builds Windows executables, runs tests on Windows and Linux, and publishes the ZIP and checksums after both jobs pass. The release appears when that workflow finishes. You do not need Go or Python installed on your PC to use this publishing path.

GitHub CLI configures Git authentication for GitHub. The script sets author information only in the local project when it is missing, using the account's GitHub no-reply address. Existing author settings are used when present.

To publish the source without creating the release tag:

```powershell
.\publish.cmd -SkipRelease
```

If the script reports an account mismatch, switch accounts with `gh auth switch --hostname github.com --user kimineechan`. If workflow files cannot be pushed because the CLI token lacks permission, run `gh auth refresh --hostname github.com --scopes workflow` and retry.

The script can reuse a matching repository after a partially completed attempt. Conflicting remote history is rejected rather than overwritten.

## Alternative: upload using GitHub's website

1. Create a public repository named **LazarusBinder** at [github.com/new](https://github.com/new). Paste the description above. Leave README, licence, and gitignore initialization unchecked because these files are included.
2. Upload the **contents** of the extracted `LazarusBinder` folder, including `.github`, `assets`, `cmd`, `internal`, `scripts`, `docs`, and root files. Omit `dist`, generated EXEs, and ZIP files. Keep `go.mod` and `README.md` at the repository root.
3. Add the topics above in the repository's About settings.
4. Check the first **Build and release** run in Actions.
5. Create a release with tag **v1.0.0**, target **main**, and title **LazarusBinder v1.0.0**.
6. Paste the contents of `docs/releases/v1.0.0.md`.
7. Attach `dist/LazarusBinder-v1.0.0-windows-x64.zip` and `dist/SHA256SUMS.txt`, then publish the release.

The prepared ZIP contains the same Windows EXE that was tested successfully. It is an asset for Releases; it is not a replacement for uploading the source repository.

## Future releases

1. Change `VERSION`, add a changelog entry, and create `docs/releases/v<version>.md`.
2. Commit and push `main`.
3. Create and push the matching version tag:

```powershell
git tag -a v1.0.1 -m "LazarusBinder v1.0.1"
git push origin v1.0.1
```

The tag must match `VERSION`. Each tagged release uses source at that tag. Update versions instead of moving an existing release tag.

If the workflow fails, open the failing job in Actions, fix the error, and rerun it. A rerun after an already-published release can be handled by uploading the rebuilt assets manually; the workflow does not overwrite an existing release.

## Reference

- [GitHub CLI repository creation](https://cli.github.com/manual/gh_repo_create)
- [GitHub CLI repository settings](https://cli.github.com/manual/gh_repo_edit)
- [GitHub CLI release creation](https://cli.github.com/manual/gh_release_create)

