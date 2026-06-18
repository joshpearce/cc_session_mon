{
  description = "Claude Code session monitoring TUI";

  inputs = {
    flake-parts.url = "github:hercules-ci/flake-parts";
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
  };

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
      }: let
        # Release builds write the tag (without the leading "v") to .version in
        # CI; local/dev builds fall back to a -dev marker. The file is gitignored.
        version = let
          versionFile = ./. + "/.version";
        in
          if builtins.pathExists versionFile
          then builtins.replaceStrings ["\n"] [""] (builtins.readFile versionFile)
          else "0.1.0-dev";
      in {
        overlayAttrs = {
          inherit (config.packages) cc-session-mon;
        };
        packages = {
          default = config.packages.cc-session-mon;
          cc-session-mon = pkgs.buildGo125Module {
            pname = "cc-session-mon";
            inherit version;
            vendorHash = builtins.readFile ./cc-session-mon.sri;
            src = lib.sourceFilesBySuffices (lib.sources.cleanSource ./.) [
              ".go"
              ".mod"
              ".sum"
            ];
            ldflags = [
              "-s"
              "-w"
              "-X main.version=${version}"
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
}
