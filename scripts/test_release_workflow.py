"""Critical controller/source-binding tests; gh calls are mocked, never dispatched."""
import json
import os
from pathlib import Path
import tempfile
import textwrap
import unittest
from unittest.mock import patch

WORKFLOW = Path(__file__).resolve().parent.parent / ".github/workflows/release.yml"


def inline_script(step):
    section = WORKFLOW.read_text().split("name: " + step, 1)[1]
    return textwrap.dedent(section.split("          python3 - <<'PY'\n", 1)[1].split("\n          PY", 1)[0])


class ReleaseSourceBinding(unittest.TestCase):
    def test_existing_annotated_tag_is_resolved_without_writes(self):
        with tempfile.TemporaryDirectory() as tmp:
            output = Path(tmp) / "output"
            commit = "a" * 40
            calls = []

            def gh(argv, **kwargs):
                calls.append(argv)
                self.assertEqual(argv[:2], ["gh", "api"])
                self.assertEqual(len(argv), 3)  # GET only; no mutation flags/body.
                path = argv[2]
                if path.endswith("/git/ref/tags/v0.1.0-beta.1"):
                    return json.dumps({"object": {"type": "tag", "sha": "b" * 40}})
                self.assertTrue(path.endswith("/git/tags/" + "b" * 40))
                return json.dumps({"object": {"type": "commit", "sha": commit}})

            with patch.dict(os.environ, {"RELEASE_TAG": "v0.1.0-beta.1", "GITHUB_REPOSITORY": "fixture/repo", "GITHUB_OUTPUT": str(output)}), patch("subprocess.check_output", gh):
                exec(inline_script("Resolve immutable existing tag"), {})
            self.assertEqual(output.read_text(), "tag=v0.1.0-beta.1\ncommit=" + commit + "\n")
            self.assertEqual(len(calls), 2)

    def test_invalid_tag_refuses_before_api_or_output(self):
        for tag in ["main", "../v0.1.0", "v0.1.0\ncommit=forged", "v0.1.0;cmd"]:
            with patch.dict(os.environ, {"RELEASE_TAG": tag}), patch("subprocess.check_output") as gh:
                with self.assertRaises(AssertionError):
                    exec(inline_script("Resolve immutable existing tag"), {})
                gh.assert_not_called()

    def test_publication_rechecks_tag_commit(self):
        code = inline_script("Publish on GitHub Releases only")
        for actual, passes in [("a" * 40, True), ("b" * 40, False)]:
            def gh(argv, **kwargs):
                self.assertEqual(argv[:2], ["gh", "api"])
                return json.dumps({"object": {"type": "commit", "sha": actual}})
            with patch.dict(os.environ, {"RELEASE_TAG": "v0.1.0-beta.1", "EXPECTED_COMMIT": "a" * 40, "GITHUB_REPOSITORY": "fixture/repo"}), patch("subprocess.check_output", gh):
                if passes:
                    exec(code, {})
                else:
                    with self.assertRaises(AssertionError):
                        exec(code, {})


if __name__ == "__main__":
    unittest.main()
