{
  description = "PROVES CI OS development shell";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";

  outputs =
    { nixpkgs, ... }:
    let
      systems = [
        "x86_64-linux"
        "aarch64-linux"
        "x86_64-darwin"
        "aarch64-darwin"
      ];
      forAllSystems = f: nixpkgs.lib.genAttrs systems (system: f nixpkgs.legacyPackages.${system});
    in
    {
      devShells = forAllSystems (pkgs: {
        default = pkgs.mkShell {
          packages = with pkgs; [
            gnumake
            hadolint
            container-structure-test
            pre-commit
            go
            golangci-lint
            cosign
            skopeo
            renovate
            rumdl
            codespell
          ];
          # Install git hooks on entry. podman is installed separately (a Linux VM on macOS).
          shellHook = ''
            if [ -d .git ]; then pre-commit install --install-hooks >/dev/null; fi
          '';
        };
      });
    };
}
