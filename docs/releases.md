# CI and GitHub prereleases

Distribution is GitHub Releases only. No registry, installer, package-manager tap or autoupdater. Adding these workflows does not create a tag or publish a release.

## CI

PRs and pushes to main run `.github/workflows/ci.yml`:

- Go pinned by `.go-version` (currently 1.26.5); no local toolchain/PATH changes. CI setup installs that toolchain on ephemeral runners.
- Native hosted macOS Intel (`macos-15-intel`) and Linux vet + race tests; Windows vet + ordinary tests, including explicit protected-storage refusal. Keyring adapters use mocks, OAuth tests use fixtures, not a real Google grant.
- Compiled discovery, strict-input and local chat-approval smoke. Windows checks discovery and fails closed for protected state. No login, host keyring or provider calls.
- Six CGO-disabled cross-builds, archive/checksum checks, and packaged Linux amd64 smoke with injected version. Cross-builds are not native runtime evidence for every architecture.

macOS CI is pinned to the standard Intel runner because `macos-latest` selected an arm64 pool that could not acquire a runner during release recovery. This changes the test host, not the test commands or six release targets. Darwin arm64 remains a cross-build target; passing Intel CI is not native arm64 CI proof.

The CI tag `v0.0.0-ci` is only an injected binary label for branch/PR packaging, never a git tag or published release. Python 3.12 is a CI/build-time tool for stdlib packaging/tests, not a runtime dependency of Sela.

## Publish a prerelease

After reviewing CI and approving a release version, a maintainer deliberately creates and pushes a SemVer tag, for example:

```sh
# Example only. This publishes when the tag workflow finishes successfully.
git tag -a v0.1.0-alpha.1 -m 'Sela v0.1.0-alpha.1'
git push origin v0.1.0-alpha.1
```

The tag's source must contain the workflows. Pushing `v*` runs `.github/workflows/release.yml`, which reuses all CI gates against that tagged commit. Invalid version tags, dirty source and failed checks block publication. Main-push CI never publishes. A maintainer can explicitly retry an existing tag via the manual path below; it runs the same gates, not a direct-publish shortcut.

Read-only build jobs upload assets within the same workflow run. Only the final publish job has `contents:write`; it does not check out or run repository scripts. It verifies exact filenames, tag/commit, clean-source metadata and SHA256 before `gh release create --verify-tag --prerelease --latest=false`. No Google credentials, PAT or additional secrets are needed; it uses the scoped ephemeral GITHUB_TOKEN. Repository/org settings may still block Actions or token writes; fix the actual permission, never add a broad token as a workaround.

All releases are marked prerelease, even an unsuffixed `v0.1.0` tag. Stable promotion is a separate future decision. Concurrent runs for the same ref queue rather than cancel. Existing releases are not overwritten or clobbered: if publication is interrupted or a rerun finds an existing release, inspect it and reconcile manually.

## Retry an existing tag without moving it

If a tag event was missed (for example during a GitHub incident), after confirming the tag is correct and no release/run already exists:

```sh
# Publishes only after all checks succeed. Requires release approval.
gh workflow run release.yml --ref main -f tag=v0.1.0-beta.1
```

The current workflow controller on main resolves the **existing** tag to a commit. Native test jobs and packaging explicitly check out that tag, verify the expected commit before running project code, and build that source—not main's newer code under an old version label. Artifact inventory/checksums bind version and commit. The publisher rechecks the tag's commit immediately before creation and refuses a moved tag. No tag is created, deleted, retargeted or force-pushed. Publication deliberately omits `--target`: the tag already exists and its commit is checked independently; sending a redundant target override can trigger a workflow-permission 403 for the Actions integration token (see [gh issue #9514](https://github.com/cli/cli/issues/9514)). No extra PAT or permission expansion is used. Missing/invalid tags fail closed. Existing releases are not overwritten; inspect/reconcile them before retrying.

This requires workflow-dispatch permission. Editing workflow files via classic OAuth also requires the `workflow` scope; source-code push permission alone may not cover that. Do not export a token or broaden its scopes without the user's approval. Git credential helper `gh auth git-credential` can be used per-command for HTTPS without changing global Git config or the SSH remote.

## Assets and support boundaries

- `sela_VERSION_darwin_{amd64,arm64}.tar.gz`
- `sela_VERSION_linux_{amd64,arm64}.tar.gz`
- `sela_VERSION_windows_{amd64,arm64}.zip`
- `MANIFEST.json`, `SHA256SUMS.txt`, `RELEASE_NOTES.md`

Each archive contains only the binary, LICENSE and README. Embedded skills travel inside the binary. No account metadata, client JSON, tokens, plans, logs or user content are packaged. Manifest records version, source commit, toolchain, target, binary/archive hashes and platform limits. `sela version` reports injected tag/commit plus build/platform metadata; ordinary source builds retain the dev version and available VCS revision.

**Windows is experimental/discovery-only.** Protected account state, login and writes refuse until private-storage ACL/locking support exists. Linux keyring integration and all OS/architecture combinations are not validated by cross-builds. macOS has the limited live evidence described in README, not broad production assurance.

Binaries are not publisher-signed or notarized. Checksums detect corruption/mismatch; they are not a substitute for publisher authentication. Download all assets from the intended GitHub release and check them with `sha256sum -c SHA256SUMS.txt` (macOS: `shasum -a 256 -c SHA256SUMS.txt`; Windows can use `Get-FileHash -Algorithm SHA256` and compare entries). macOS/Windows policy may reject untrusted binaries; use a reviewed source build or your normal security review, not an automatic bypass script.

## Local validation without publishing

```sh
make check
make smoke
python3 scripts/build_release.py --version v0.0.0-local-test --output dist
```

Output must be empty; the builder refuses to mix/overwrite assets. A dirty local tree may be packaged for inspection with `modified:true`; the publish verifier always rejects it. No builder command creates tags or publishes anything. Do not run the example tag commands just to test packaging.
