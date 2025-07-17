{
  description = "MCP Language Server";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-parts.url = "github:hercules-ci/flake-parts";
  };

  outputs = inputs@{ flake-parts, ... }:
    flake-parts.lib.mkFlake { inherit inputs; } {
      systems = [ "x86_64-linux" "aarch64-linux" "x86_64-darwin" "aarch64-darwin" ];
      
      perSystem = { config, self', inputs', pkgs, system, ... }: {
        packages.default = pkgs.buildGoModule {
          pname = "mcp-language-server";
          version = "0.0.2";
          
          src = ./.;
          
          vendorHash = "sha256-WcYKtM8r9xALx68VvgRabMPq8XnubhTj6NAdtmaPa+g=";
          
          subPackages = [ "." ];
          
          ldflags = [ "-s" "-w" ];
          
          meta = with pkgs.lib; {
            description = "MCP server that exposes Language Server Protocol capabilities to LLMs";
            homepage = "https://github.com/isaacphi/mcp-language-server";
            license = licenses.bsd3;
            maintainers = [ ];
          };
        };
        
        apps.default = {
          type = "app";
          program = "${self'.packages.default}/bin/mcp-language-server";
        };
        
        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            go
            just
            gopls
          ];
        };
      };
    };
}