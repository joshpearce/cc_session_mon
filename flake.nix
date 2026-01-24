{
  description = "Claude Code session monitoring TUI";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
      in {
        packages = {
          cc-session-mon = pkgs.buildGo125Module {
            pname = "cc-session-mon";
            version = "0.1.0";
            src = ./.;
            vendorHash = "sha256-Dv8puCTuUDT/FSY8tyegj4ZkZ3zsU88xYYLFRhX3qWU=";
            ldflags = [ "-s" "-w" ];
          };
          default = self.packages.${system}.cc-session-mon;
        };

        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [ go_1_25 gopls golangci-lint ];
        };
      }
    ) // {
      overlays.default = final: prev: {
        cc-session-mon = self.packages.${prev.system}.cc-session-mon;
      };
    };
}
