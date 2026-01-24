{
  description = "Claude Code session monitoring TUI";

  outputs = inputs @ {
    self,
    flake-parts,
    ...
  }:
    flake-parts.lib.mkFlake {inherit inputs;} {
      imports = [
        inputs.flake-parts.flakeModules.easyOverlay
        inputs.flake-parts.flakeModules.partitions
      ];
      systems = [
        "x86_64-darwin"
        "x86_64-linux"
        "aarch64-darwin"
        "aarch64-linux"
      ];
      perSystem = {
        config,
        lib,
        pkgs,
        ...
      }: {
        overlayAttrs = {
          inherit (config.packages) cc-session-mon;
        };
        packages = {
          default = config.packages.cc-session-mon;
          cc-session-mon = pkgs.buildGo125Module {
            pname = "cc-session-mon";
            version = "0.1.0";
            vendorHash = builtins.readFile ./cc-session-mon.sri;
            src = lib.sourceFilesBySuffices (lib.sources.cleanSource ./.) [
              ".go"
              ".mod"
              ".sum"
            ];
            ldflags = [
              "-s"
              "-w"
            ];
          };
        };

        formatter = pkgs.alejandra;
      };

      partitionedAttrs = {
        apps = "dev";
        checks = "dev";
        devShells = "dev";
      };
      partitions.dev = {
        extraInputsFlake = ./dev;
        module = ./dev/flake-part.nix;
      };
      flake = {
        overlays.default = inputs.self.overlays.additions;
      };
    };

  inputs = {
    flake-parts.url = "github:hercules-ci/flake-parts";
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
  };
}
