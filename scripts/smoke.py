"""Compiled CLI smoke: synthetic private state only; no login/keyring/provider calls."""
import argparse
import json
import os
from pathlib import Path
import subprocess
import tempfile


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--build", action="store_true")
    parser.add_argument("--binary", type=Path)
    parser.add_argument("--expected-version")
    args = parser.parse_args()
    repo = Path(__file__).resolve().parent.parent
    binary = (args.binary or repo / "bin" / ("sela.exe" if os.name == "nt" else "sela")).resolve()
    if args.build:
        binary.parent.mkdir(parents=True, exist_ok=True)
        subprocess.run(["go", "build", "-trimpath", "-o", str(binary), "./cmd/sela"], cwd=repo, check=True)
    with tempfile.TemporaryDirectory(prefix="sela-smoke-") as temp:
        root = Path(temp).resolve()
        if os.name != "nt":
            root.chmod(0o700)
        env = {**os.environ, "HOME": str(root), "USERPROFILE": str(root), "XDG_CONFIG_HOME": str(root / "config")}

        def run(*argv, error=None):
            result = subprocess.run([str(binary), *argv], cwd=root, env=env, stdin=subprocess.DEVNULL,
                                    capture_output=True, text=True, timeout=15)
            if error:
                assert result.returncode and error in result.stderr, (argv, result.stderr)
                return None
            assert result.returncode == 0, (argv, result.stderr)
            return result.stdout

        assert "--user-approval" in run("--help")
        for topic in [None, "gmail", "calendar", "workflows", "safety"]:
            assert len(run("--skill", *([topic] if topic else []))) > 100
        metadata = json.loads(run("version"))
        assert metadata["version"] and metadata["commit"] and metadata["platform"]
        if args.expected_version:
            assert metadata["version"] == args.expected_version
        scopes = json.loads(run("auth", "scopes"))
        assert scopes["granted"] is False and len(scopes["default"]) == 6
        assert not (root / "config").exists(), "discovery wrote state"
        run("mail", "search", "--account", "missing", error="explicit query")
        run("plan", "approve", "--account", "missing", "--id", "a" * 32, error="--user-approval")
        if os.name == "nt":
            run("account", "add", "--name", "fixture", "--email", "person@example.test", error="Windows private storage is not enabled")
        else:
            run("account", "add", "--name", "fixture", "--email", "person@example.test")
            file = root / "mail.json"
            file.write_text(json.dumps({"to": ["guest@example.test"], "subject": "Synthetic fixture", "body": "fixture"}))
            file.chmod(0o600)
            plan = json.loads(run("mail", "preview", "--account", "fixture", "--action", "send", "--file", str(file)))
            pid, digest = plan["scope"]["id"], plan["sha256"]
            run("plan", "execute", "--account", "fixture", "--id", pid, error="must be approved")
            run("plan", "approve", "--account", "fixture", "--id", pid, "--hash", "0" * 64,
                "--user-approval", "Synthetic fixture consent", error="does not match")
            approved = json.loads(run("plan", "approve", "--account", "fixture", "--id", pid, "--hash", digest,
                                     "--user-approval", "Synthetic fixture consent", "--approval-ref", "CI fixture"))
            assert approved["state"] == "approved" and approved["approval"]["plan_sha256"] == digest
            assert approved["approval"]["method"] == "agent_attested_chat" and "attempted_at" not in approved
            assert not (root / "config" / "sela" / "connections").exists()
    print("PASS compiled discovery/local guards; no live Google or credential-store validation")


if __name__ == "__main__":
    main()
