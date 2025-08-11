{
  description = "MCP Language Server - A Model Context Protocol server that runs and exposes a language server to LLMs";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
      in
      {
        packages.default = pkgs.buildGoModule rec {
          pname = "mcp-language-server";
          version = "0.1.0";

          src = ./.;

          vendorHash = "sha256-WcYKtM8r9xALx68VvgRabMPq8XnubhTj6NAdtmaPa+g=";

          # Only build the main package
          subPackages = [ "." ];

          ldflags = [
            "-s"
            "-w"
            "-X main.version=${version}"
          ];

          meta = with pkgs.lib; {
            description = "MCP server that runs and exposes a language server to LLMs";
            homepage = "https://github.com/isaacphi/mcp-language-server";
            license = licenses.bsd3;
            maintainers = [ ];
            platforms = platforms.unix;
          };

          # Skip tests for now as they require external language servers
          doCheck = false;
        };

        apps.default = {
          type = "app";
          program = "${self.packages.${system}.default}/bin/mcp-language-server";
        };

        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            go
            gopls
            golangci-lint
            delve
            just
          ];

          shellHook = ''
            echo "MCP Language Server development shell"
            echo "Available commands:"
            echo "  go build    - Build the binary"
            echo "  go test     - Run tests"
            echo "  just -l     - List available just recipes"
          '';
        };
      });
}