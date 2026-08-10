{
  description = "containerlab - container-based networking labs";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs =
    { self, nixpkgs }:
    let
      # ponytail: linux-only, matching .goreleaser.yml's goos list
      systems = [
        "x86_64-linux"
        "aarch64-linux"
      ];
      forAll = f: nixpkgs.lib.genAttrs systems (s: f nixpkgs.legacyPackages.${s});
    in
    {
      packages = forAll (pkgs: rec {
        containerlab = pkgs.buildGoModule {
          pname = "containerlab";
          version = "0.78.0";
          src = ./.;
          vendorHash = "sha256-kRBYjxirApj91hNBz3a+NyRm8SqRTVeQQCz+JFsKY0U=";

          tags = [
            "podman"
            "exclude_graphdriver_btrfs"
            "btrfs_noversion"
            "exclude_graphdriver_devicemapper"
            "exclude_graphdriver_overlay"
            "containers_image_openpgp"
          ];
          ldflags = [
            "-s"
            "-w"
            "-X github.com/srl-labs/containerlab/cmd.Version=${containerlab.version}"
          ];

          # ponytail: tests need docker/root, skip them here
          doCheck = false;

          meta = {
            description = "Container-based networking labs";
            homepage = "https://containerlab.dev";
            license = pkgs.lib.licenses.bsd3;
            mainProgram = "containerlab";
          };
        };
        default = containerlab;
      });

      devShells = forAll (pkgs: {
        default = pkgs.mkShell {
          packages = [
            pkgs.go
            pkgs.gopls
            pkgs.golangci-lint
          ];
        };
      });
    };
}
