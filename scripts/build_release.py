"""Build local release archives only. This script never creates tags or publishes."""
import argparse
import datetime
import gzip
import hashlib
import io
import json
import os
from pathlib import Path
import re
import subprocess
import tarfile
import zipfile

TARGETS = [(osname, arch) for osname in ("darwin", "linux", "windows") for arch in ("amd64", "arm64")]
VERSION = re.compile(r"v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?")


def validate_version(value):
    match = VERSION.fullmatch(value)
    if not match or any(p.isdigit() and len(p) > 1 and p.startswith("0") for p in (match[4] or "").split(".")):
        raise ValueError("version must be a safe SemVer tag, e.g. v0.1.0-alpha.1")
    return value


def sha256(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def archive(path, files, epoch):
    if path.suffix == ".zip":
        date = datetime.datetime.fromtimestamp(max(epoch, 315532800), datetime.timezone.utc).timetuple()[:6]
        with zipfile.ZipFile(path, "w", compression=zipfile.ZIP_DEFLATED) as output:
            for name, data, mode in files:
                entry = zipfile.ZipInfo(name, date)
                entry.create_system = 3
                entry.external_attr = (0o100000 | mode) << 16
                entry.compress_type = zipfile.ZIP_DEFLATED
                output.writestr(entry, data)
    else:
        with path.open("wb") as raw, gzip.GzipFile(filename="", mode="wb", fileobj=raw, mtime=epoch) as compressed:
            with tarfile.open(fileobj=compressed, mode="w") as output:
                for name, data, mode in files:
                    entry = tarfile.TarInfo(name)
                    entry.size, entry.mode, entry.mtime = len(data), mode, epoch
                    output.addfile(entry, io.BytesIO(data))


def verify(directory, version, commit, require_clean=True):
    validate_version(version)
    manifest = json.loads((directory / "MANIFEST.json").read_text())
    if manifest["version"] != version or manifest["commit"] != commit or (require_clean and manifest["modified"]):
        raise ValueError("release identity must match a clean tagged commit")
    expected = {f"sela_{version}_{osname}_{arch}" + (".zip" if osname == "windows" else ".tar.gz") for osname, arch in TARGETS}
    expected |= {"MANIFEST.json", "RELEASE_NOTES.md"}
    if {p.name for p in directory.iterdir()} != expected | {"SHA256SUMS.txt"}:
        raise ValueError("unexpected/missing release files")
    checksums = {}
    for line in (directory / "SHA256SUMS.txt").read_text().splitlines():
        digest, name = line.split("  ", 1)
        if name not in expected or name in checksums or digest != sha256(directory / name):
            raise ValueError("release checksum/name mismatch")
        checksums[name] = digest
    if set(checksums) != expected:
        raise ValueError("incomplete checksums")
    return manifest


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--version", required=True, type=validate_version)
    parser.add_argument("--output", type=Path, default=Path("dist"))
    parser.add_argument("--go", default="go")
    args = parser.parse_args()
    repo = Path(__file__).resolve().parent.parent
    out = args.output.resolve()
    if out.exists() and any(out.iterdir()):
        raise SystemExit("output directory must be empty; refusing to mix/overwrite assets")
    out.mkdir(parents=True, exist_ok=True)
    git = lambda *a: subprocess.check_output(["git", *a], cwd=repo, text=True).strip()
    commit, epoch = git("rev-parse", "HEAD"), int(git("show", "-s", "--format=%ct", "HEAD"))
    modified = bool(git("status", "--porcelain"))
    manifest = {"version": args.version, "commit": commit, "modified": modified, "go": subprocess.check_output([args.go, "version"], text=True).strip(), "artifacts": []}
    scratch = out / ".build"
    scratch.mkdir()
    try:
        for osname, arch in TARGETS:
            name = "sela.exe" if osname == "windows" else "sela"
            binary = scratch / name
            env = {**os.environ, "GOOS": osname, "GOARCH": arch, "CGO_ENABLED": "0", "GOTOOLCHAIN": "local"}
            flags = f"-X github.com/ans-4175/sela/internal/cli.version={args.version} -X github.com/ans-4175/sela/internal/cli.commit={commit}"
            subprocess.run([args.go, "build", "-trimpath", "-buildvcs=true", "-ldflags", flags, "-o", str(binary), "./cmd/sela"], cwd=repo, env=env, check=True)
            filename = f"sela_{args.version}_{osname}_{arch}" + (".zip" if osname == "windows" else ".tar.gz")
            files = [(name, binary.read_bytes(), 0o755), *[(p, (repo / p).read_bytes(), 0o644) for p in ("LICENSE", "README.md")]]
            archive(out / filename, files, epoch)
            manifest["artifacts"].append({"os": osname, "arch": arch, "file": filename, "sha256": sha256(out / filename), "binary_sha256": sha256(binary), "scope": "experimental discovery-only; protected state disabled" if osname == "windows" else "build artifact; see README runtime-validation limits"})
            binary.unlink()
    finally:
        for path in scratch.iterdir():
            path.unlink()
        scratch.rmdir()
    (out / "MANIFEST.json").write_text(json.dumps(manifest, indent=2) + "\n")
    (out / "RELEASE_NOTES.md").write_text(f"""# Sela {args.version} — prerelease

Commit: `{commit}`

Six build targets: macOS/Darwin, Linux, Windows × amd64/arm64. Cross-build is not runtime validation.

**Windows is experimental, discovery-only. Account storage, login and approved writes deliberately fail closed pending private-storage ACL/locking support.**

CI uses synthetic fixtures and mock credentials only, never live Google or host keyring operations. Linux keyring integration and every OS/architecture combination are not established by these tests. macOS has limited live evidence documented in README, not a production-readiness guarantee.

The agent records real user chat consent; the CLI cannot independently authenticate that chat. OAuth consent is separate. No user credentials/config/state are bundled.

Assets have SHA256 checksums; this is integrity checking, not publisher signing. Binaries are not publisher-signed or notarized. No package registry, installer, Homebrew or autoupdater is involved.
""")
    files = sorted(out.iterdir())
    (out / "SHA256SUMS.txt").write_text("".join(f"{sha256(p)}  {p.name}\n" for p in files))
    verify(out, args.version, commit, require_clean=not modified)
    print(f"Built {len(TARGETS)} archives in {out}; modified={modified}; nothing published")


if __name__ == "__main__":
    main()
