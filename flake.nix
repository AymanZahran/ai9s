{
  description = "Keyboard-first finder for local AI coding sessions";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-24.11";

  outputs = { self, nixpkgs }:
    let
      version = "1.0.2";
      systems = {
        x86_64-linux = {
          asset = "ai9s_Linux_amd64.tar.gz";
          hash = "sha256-bI9uAdO5vcLU1SyVNbpqYR+LquDo6Bd8HCyf/1T2LWA=";
        };
        aarch64-linux = {
          asset = "ai9s_Linux_arm64.tar.gz";
          hash = "sha256-X0xbkIDu5Hwx+O20iXQXiPpsZCmEbKxEkmN9pureq+M=";
        };
        x86_64-darwin = {
          asset = "ai9s_Darwin_amd64.tar.gz";
          hash = "sha256-fjsA5RzB1zq+tpSZmAwi7na0u2g5n0/uve9+d+/Z4SE=";
        };
        aarch64-darwin = {
          asset = "ai9s_Darwin_arm64.tar.gz";
          hash = "sha256-9zkz07xbcnyOr4C27glnLCGKCw9ZrBF0rt4Gydv1B4Y=";
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
