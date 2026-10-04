{
  description = "Keyboard-first finder for local AI coding sessions";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-24.11";

  outputs = { self, nixpkgs }:
    let
      version = "1.0.4";
      systems = {
        x86_64-linux = {
          asset = "ai9s_Linux_amd64.tar.gz";
          hash = "sha256-8C5ACM8terzOu75J15jOvaIA68+LLWMMf5cqickcOhY=";
        };
        aarch64-linux = {
          asset = "ai9s_Linux_arm64.tar.gz";
          hash = "sha256-rOQGxdqHFET5bbLmsDHy8WYLw0hRRxx2b5P9SO8wyTU=";
        };
        x86_64-darwin = {
          asset = "ai9s_Darwin_amd64.tar.gz";
          hash = "sha256-goCmSoxXAIIyT8quIyrtiNoDreLfI5UbeSBk8vov5LI=";
        };
        aarch64-darwin = {
          asset = "ai9s_Darwin_arm64.tar.gz";
          hash = "sha256-jyfEUE5nU9JIrSPoqWtIfw2jqixUiNrVvBTu0ax1TJM=";
        };
      };
      packagesFor = nixpkgs.lib.mapAttrs (system: src:
        let
          pkgs = import nixpkgs { inherit system; };
        in {
          default = pkgs.stdenv.mkDerivation {
            pname = "ai9s";
            inherit version;
            src = pkgs.fetchurl {
              url = "https://github.com/AymanZahran/ai9s/releases/download/v${version}/${src.asset}";
              hash = src.hash;
            };
            dontUnpack = true;
            installPhase = ''
              mkdir -p "$out/bin"
              tar -xzf "$src" -C "$out/bin" ai9s
              chmod 755 "$out/bin/ai9s"
            '';
            meta = {
              description = "Keyboard-first finder for local AI coding sessions";
              homepage = "https://ai9scli.io";
              license = pkgs.lib.licenses.mit;
              mainProgram = "ai9s";
              platforms = [ system ];
            };
          };
        });
    in {
      packages = packagesFor systems;
    };
}
