# Verifying releases

Every release is built by GitHub Actions from the tagged commit
(`.github/workflows/release.yml`). Nothing is uploaded by hand. You can
check a download in three increasingly thorough ways.

## 1. Checksums

Each release has a `SHA256SUMS` file. In the folder with your downloads:

```sh
sha256sum --ignore-missing -c SHA256SUMS            # Linux
shasum -a 256 -c SHA256SUMS 2>&1 | grep -v 'No such'  # macOS
```

On Windows (PowerShell), compare the result with the line in `SHA256SUMS`:

```powershell
Get-FileHash .\rp2350-gamepad-v1.0.0.uf2 -Algorithm SHA256
```

A matching checksum proves the file wasn't damaged or swapped after
`SHA256SUMS` was created. It doesn't prove who made it; for that, see the
next step.

## 2. Build provenance (who built it, and from what)

Every file is covered by a signed
[build provenance attestation](https://docs.github.com/actions/security-for-github-actions/using-artifact-attestations).
It records which repository, workflow, commit and tag produced the file,
and is signed through GitHub's Sigstore integration. Verify it with the
[GitHub CLI](https://cli.github.com/) (`gh`):

```sh
gh attestation verify rp2350-gamepad-v1.0.0.uf2 --repo gethiox/rp2350-gamepad
```

A successful result looks like this:

```
✓ Verification succeeded!

The following 1 attestation matched the policy criteria
- Attestation #1
  - Build repo:..... gethiox/rp2350-gamepad
  - Build workflow:. .github/workflows/release.yml@refs/tags/v1.0.0
  - Signer repo:.... gethiox/rp2350-gamepad
  - Signer workflow: .github/workflows/release.yml@refs/tags/v1.0.0
```

Check that *Build workflow* names `release.yml` and the tag you expect. To
verify every file of a release at once:

```sh
gh release download v1.0.0 --repo gethiox/rp2350-gamepad --dir rp2350-gamepad-v1.0.0
cd rp2350-gamepad-v1.0.0
for f in *; do gh attestation verify "$f" --repo gethiox/rp2350-gamepad || echo "FAILED: $f"; done
```

To be stricter, require that the file came from that exact tag and
workflow:

```sh
gh attestation verify rp2350-gamepad-v1.0.0.uf2 \
  --repo gethiox/rp2350-gamepad \
  --signer-workflow gethiox/rp2350-gamepad/.github/workflows/release.yml \
  --source-ref refs/tags/v1.0.0
```

`gh attestation verify` needs network access and `gh auth login` (any
GitHub account works).

## 3. Rebuild the firmware yourself

The firmware build is reproducible: the same commit always produces
byte-identical `.uf2` and `.elf` files. The build pins the Pico SDK commit,
the exact Arm toolchain (verified by SHA-256) and the timestamp
(`SOURCE_DATE_EPOCH` = commit time), and strips build paths. On Linux
x86-64:

```sh
git clone https://github.com/gethiox/rp2350-gamepad
cd rp2350-gamepad
git checkout v1.0.0
GP_VERSION=v1.0.0 scripts/build-firmware.sh
sha256sum dist/rp2350-gamepad-v1.0.0.uf2   # compare with SHA256SUMS of the release
```

The configurator binaries are built with `-trimpath` but are **not**
guaranteed to be bit-identical: they link the platform's C compiler and
system libraries. Use steps 1 and 2 for them.

## Immutable releases (for the maintainer)

Turn on GitHub's *immutable releases* option in the repository settings
(*Settings → General*, "Releases" section). Once a release is published,
its assets and tag can then no longer be changed or replaced, not even by
the maintainer. If a release turns out to be broken, publish a new
version instead.
