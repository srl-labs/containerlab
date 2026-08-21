{
  description = "containerlab - container-based networking labs";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs =
    { self, nixpkgs }:
    let
      # linux-only, matching .goreleaser.yml's goos list
      systems = [
        "x86_64-linux"
        "aarch64-linux"
      ];
      forAll = f: nixpkgs.lib.genAttrs systems (s: f nixpkgs.legacyPackages.${s});
      version = "0.79.0";
    in
    {
      packages = forAll (pkgs: rec {
        containerlab = pkgs.buildGoModule {
          pname = "containerlab";
          inherit version;
          src = ./.;
          vendorHash = "sha256-Z5AA/CuxT/9ZDhfZ7JOh19gb9nzIFHRP89F6xr+bUa0=";
          env = {
            CGO_ENABLED = 0;
          };

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
            "-X github.com/srl-labs/containerlab/cmd.Version=${version}"
            "-X github.com/srl-labs/containerlab/cmd.commit=${self.shortRev or "unknown"}"
          ];

          # stamp the build date (ISO 8601, UTC) of the flake revision,
          # which keeps the build reproducible for a given input.
          # note: self.lastModified (epoch) is used instead of
          # self.lastModifiedDate, whose format varies across nix versions.
          preBuild = ''
            ldflags+=" -X github.com/srl-labs/containerlab/cmd.date=$(date -u -d @${toString self.lastModified} +%Y-%m-%dT%H:%M:%SZ)"
          '';

          # tests need docker/root, skip them here
          doCheck = false;

          meta = {
            description = "Container-based networking labs";
            homepage = "https://containerlab.dev";
            license = nixpkgs.lib.licenses.bsd3;
            mainProgram = "containerlab";
            platforms = nixpkgs.lib.platforms.linux;
          };
        };
        default = containerlab;
      });

      nixosModules.default =
        {
          config,
          lib,
          pkgs,
          ...
        }:
        let
          cfg = config.programs.containerlab;
        in
        {
          options.programs.containerlab = {
            enable = lib.mkEnableOption "containerlab";
            package = lib.mkOption {
              type = lib.types.package;
              default = self.packages.${pkgs.system}.containerlab;
              description = "The containerlab package to install.";
            };
          };

          config = lib.mkIf cfg.enable {
            environment.systemPackages = [ cfg.package ];

            security.wrappers.containerlab = {
              source = "${cfg.package}/bin/containerlab";
              owner = "root";
              group = "root";
              setuid = true;
            };

            security.wrappers.clab = {
              source = "${cfg.package}/bin/containerlab";
              owner = "root";
              group = "root";
              setuid = true;
            };

            users.groups.clab_admins = { };
          };
        };

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
