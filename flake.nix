{
  description = "AgentMemory Universal (v3.0 Cognitive Engine) - Pure Go Edition";

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
        packages.default = pkgs.buildGoModule {
          pname = "agent-memory";
          version = "3.0.0";
          src = ./.;

          vendorHash = null;

          env = {
            CGO_ENABLED = 0;
          };

          ldflags = [
            "-s"
            "-w"
          ];

          subPackages = [ "cmd/agent-memory" ];

          meta = with pkgs.lib; {
            description = "AgentMemory Universal (v3.0) Cognitive Memory Engine";
            homepage = "https://github.com/surtr85/agent-memory";
            license = licenses.mit;
          };
        };

        devShells.default = pkgs.mkShell {
          packages = with pkgs; [
            go
            gopls
            gotools
            sqlite
          ];

          shellHook = ''
            echo "🧠 AgentMemory Universal (Pure Go) dev environment loaded."
          '';
        };
      }
    );
}
