{
  description = "MCP Language Server with Coq support";
  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs = { self, nixpkgs }:
    let
      systems = [ "x86_64-linux" "x86_64-darwin" "aarch64-darwin" ];
      forAllSystems = f: nixpkgs.lib.genAttrs systems (system:
        let pkgs = import nixpkgs { inherit system; };
        in f pkgs);
    in {
      packages = forAllSystems (pkgs: {
        default = pkgs.buildGoModule {
          pname = "mcp-language-server";
          version = "unstable-coq";
          src = ./.;
          vendorHash = "sha256-WcYKtM8r9xALx68VvgRabMPq8XnubhTj6NAdtmaPa+g=";
          subPackages = [ "." ];
        };
      });
    };
}
