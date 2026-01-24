{
  self,
  inputs,
  ...
}: {
  imports = [
    inputs.devshell.flakeModule
    inputs.generate-go-sri.flakeModules.default
  ];
  systems = [
    "x86_64-linux"
    "aarch64-darwin"
  ];

  perSystem = {
    config,
    pkgs,
    ...
  }: {
    go-sri-hashes.cc-session-mon = {};

    devshells.default = {
      commands = [
        {
          name = "regenSRI";
          category = "dev";
          help = "Regenerate cc-session-mon.sri in case the module SRI hash should change";
          command = "${config.apps.generate-sri-cc-session-mon.program}";
        }
      ];
      packages = [
        pkgs.go_1_25
        pkgs.gopls
        pkgs.golangci-lint
      ];
    };
  };
}
