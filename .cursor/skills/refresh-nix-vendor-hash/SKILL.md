---
name: refresh-nix-vendor-hash
description: Refreshes Nix buildGoModule vendorHash or vendorSha256 values after Go dependency changes. Use when go.mod or go.sum changes, Nix reports a vendor hash mismatch, flake.nix metadata is stale, or a containerized Nix build is needed.
---

# Refresh Nix Vendor Hash

Refresh the Go dependency hash used by `buildGoModule`, then verify the
package build. This is a metadata refresh, not a reason to change `flake.lock`.

## When to run

Run this workflow when:

- `go.mod` or `go.sum` changes.
- `nix build .#containerlab --no-link` reports a fixed-output hash mismatch.
- The repository's Nix workflow reports a `got: sha256-...` value.
- A dependency upgrade changes the vendored Go module set.

Do not replace the hash when the Nix error is a compiler, test, fetch, or
flake evaluation failure. Fix that failure first.

## Preferred workflow

1. In `flake.nix`, temporarily replace the existing `vendorHash` value with:

   ```nix
   vendorHash = "sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=";
   ```

2. Run the build and save its output:

   ```bash
   nix build .#containerlab --no-link 2>&1 \
     | tee /tmp/containerlab-nix-build.log || true
   ```

3. Extract the reported hash without assuming `rg` is installed:

   ```bash
   awk '/got:/{hash=$2} END{if (hash) print hash}' \
     /tmp/containerlab-nix-build.log
   ```

4. Replace the fake value in `flake.nix` with the extracted `sha256-...`
   value. The first build is expected to fail with the fake hash.

5. Verify the real hash:

   ```bash
   nix build .#containerlab --no-link
   git diff --check
   ```

Update only `flake.nix` for a vendor hash refresh. Do not update `flake.lock`
unless the nixpkgs input also changed.

## Containerized Nix fallback

Use this when the host has no Nix installation. Mount the repository at a
path and mark it as a Git safe directory: containerized Nix commonly runs as
root while the mounted repository is owned by another user.

```bash
docker run --rm -it \
  -v "$PWD:/workspace" \
  -w /workspace \
  nixos/nix:latest \
  sh -lc '
    git config --global --add safe.directory /workspace
    nix --extra-experimental-features "nix-command flakes" \
      build .#containerlab --no-link --option sandbox false
  ' 2>&1 | tee /tmp/containerlab-nix-build.log || true
```

Then extract the hash on the host with the `awk` command above, update
`flake.nix`, and rerun the container command without `|| true`.

If Docker is unavailable, use the equivalent Podman command. If the container
cannot reach the network, configure the container runtime before diagnosing
the hash.

## Final checks

- The build succeeds with the real hash.
- `flake.nix` contains the new hash and no fake hash.
- `flake.lock` is unchanged unless nixpkgs was intentionally updated.
- `git diff --check` passes.
- Leave changes uncommitted unless the user explicitly requests a commit.
