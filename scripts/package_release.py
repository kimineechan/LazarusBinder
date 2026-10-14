#!/usr/bin/env python3
"""Package the Windows builder and documentation as a GitHub release asset."""

import argparse
import hashlib
import os
from pathlib import Path
import re
import tempfile
import zipfile


def sha256(path):
    digest = hashlib.sha256()
    with path.open("rb") as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b""):
            digest.update(block)
    return digest.hexdigest()


def main():
    root = Path(__file__).resolve().parents[1]
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--tag", help="Require the tag to match VERSION, such as v1.0.0")
    parser.add_argument("--output-dir", type=Path, default=root / "dist")
    args = parser.parse_args()
    version = (root / "VERSION").read_text(encoding="utf-8").strip()
    if not re.fullmatch(r"[0-9]+\.[0-9]+\.[0-9]+(?:-[A-Za-z0-9]+(?:[.-][A-Za-z0-9]+)*)?", version):
        parser.error("VERSION must contain a semantic version such as 1.0.0.")
    if args.tag is not None and args.tag != "v" + version:
        parser.error("The release tag must match VERSION (expected v" + version + ").")

    exe = root / "dist" / "LazarusBinder.exe"
    notes = root / "docs" / "releases" / ("v" + version + ".md")
    files = [
        ("LazarusBinder.exe", exe),
        ("README.md", root / "README.md"),
        ("LICENSE", root / "LICENSE"),
        ("THIRD-PARTY-NOTICES.txt", root / "THIRD-PARTY-NOTICES.txt"),
        ("VERSION", root / "VERSION"),
        ("CHANGELOG.md", root / "CHANGELOG.md"),
        ("VALIDATION.md", root / "VALIDATION.md"),
        ("CONTRIBUTING.md", root / "CONTRIBUTING.md"),
        ("docs/USAGE.md", root / "docs/USAGE.md"),
        ("docs/GITHUB.md", root / "docs/GITHUB.md"),
        ("docs/releases/v" + version + ".md", notes),
        ("assets/logo.svg", root / "assets/logo.svg"),
        ("assets/lazarusbinder.ico", root / "assets/lazarusbinder.ico"),
    ]
    for _, source in files:
        if not source.is_file():
            parser.error("Missing release input: " + str(source))
    if exe.stat().st_size == 0:
        parser.error("LazarusBinder.exe is empty. Build the app first.")

    output = args.output_dir.resolve()
    output.mkdir(parents=True, exist_ok=True)
    zip_path = output / ("LazarusBinder-v" + version + "-windows-x64.zip")
    exe_hash = sha256(exe)
    with tempfile.NamedTemporaryFile(prefix="lazarusbinder-release-", suffix=".zip", dir=output, delete=False) as stream:
        temporary = Path(stream.name)
    try:
        with zipfile.ZipFile(temporary, "w", zipfile.ZIP_DEFLATED, compresslevel=7) as archive:
            for name, source in files:
                archive.write(source, "LazarusBinder/" + name)
            archive.writestr("LazarusBinder/SHA256SUMS.txt", exe_hash + "  LazarusBinder.exe\n")
        with zipfile.ZipFile(temporary) as archive:
            damaged = archive.testzip()
            if damaged is not None:
                raise ValueError("Damaged ZIP entry: " + damaged)
        os.replace(temporary, zip_path)
    finally:
        temporary.unlink(missing_ok=True)

    checksums = (
        sha256(zip_path) + "  " + zip_path.name + "\n"
        + exe_hash + "  LazarusBinder.exe\n"
    )
    (output / "SHA256SUMS.txt").write_bytes(checksums.encode("ascii"))
    print("Created " + str(zip_path))
    print(checksums, end="")


if __name__ == "__main__":
    main()

