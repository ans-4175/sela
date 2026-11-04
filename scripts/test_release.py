import json
from pathlib import Path
import tarfile
import tempfile
import unittest
import zipfile

from build_release import TARGETS, archive, sha256, validate_version, verify


class ReleaseContract(unittest.TestCase):
    def test_version_never_becomes_shell_or_path_input(self):
        for value in ["v0.1.0", "v1.2.3-alpha.1+build.2"]:
            self.assertEqual(validate_version(value), value)
        for value in ["../v0.1.0", "v0.1.0;curl", "v01.2.3", "v0.1.0-01", "v0.1", "v0.1.0\n"]:
            with self.assertRaises(ValueError):
                validate_version(value)

    def test_archives_include_only_explicit_files_and_are_repeatable(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            files = [("sela", b"synthetic binary", 0o755), ("LICENSE", b"MIT", 0o644), ("README.md", b"fixture", 0o644)]
            for suffix in [".tar.gz", ".zip"]:
                first, second = root / ("a" + suffix), root / ("b" + suffix)
                archive(first, files, 1700000000)
                archive(second, files, 1700000000)
                self.assertEqual(sha256(first), sha256(second))
                if suffix == ".zip":
                    with zipfile.ZipFile(first) as result:
                        self.assertEqual(set(result.namelist()), {f[0] for f in files})
                        self.assertEqual(result.read("sela"), b"synthetic binary")
                else:
                    with tarfile.open(first) as result:
                        self.assertEqual(set(result.getnames()), {f[0] for f in files})
                        self.assertEqual(result.getmember("sela").mode, 0o755)

    def test_release_inventory_hash_identity_and_dirty_refusals(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            tag, commit = "v0.1.0-test", "a" * 40
            for system, arch in TARGETS:
                (root / (f"sela_{tag}_{system}_{arch}" + (".zip" if system == "windows" else ".tar.gz"))).write_bytes(b"fixture")
            manifest = {"version": tag, "commit": commit, "modified": False}
            (root / "MANIFEST.json").write_text(json.dumps(manifest))
            (root / "RELEASE_NOTES.md").write_text("Windows discovery-only fixture")

            def checksums():
                (root / "SHA256SUMS.txt").write_text("".join(f"{sha256(p)}  {p.name}\n" for p in sorted(root.iterdir()) if p.name != "SHA256SUMS.txt"))

            checksums()
            self.assertEqual(verify(root, tag, commit), manifest)
            with self.assertRaises(ValueError):
                verify(root, tag, "b" * 40)
            manifest["modified"] = True
            (root / "MANIFEST.json").write_text(json.dumps(manifest))
            checksums()
            with self.assertRaises(ValueError):
                verify(root, tag, commit)
            verify(root, tag, commit, require_clean=False)
            (root / "RELEASE_NOTES.md").write_text("tampered")
            with self.assertRaises(ValueError):
                verify(root, tag, commit, require_clean=False)
            checksums()
            (root / "unexpected-token.json").write_text("synthetic extra file")
            with self.assertRaises(ValueError):
                verify(root, tag, commit, require_clean=False)


if __name__ == "__main__":
    unittest.main()
