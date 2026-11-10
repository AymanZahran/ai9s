{
  description = "Keyboard-first finder for local AI coding sessions";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-24.11";

  outputs = { self, nixpkgs }:
    let
      version = "1.0.11";
      systems = {
        x86_64-linux = {
          asset = "ai9s_Linux_amd64.tar.gz";
          hash = "sha256-AJGuvEojfqyHixO9eTZLOBtgCHbRsAXLf2rPvOjpN88=";
        };
        aarch64-linux = {
          asset = "ai9s_Linux_arm64.tar.gz";
          hash = "sha256-n2/8+aI2Jk6zrG3XWNv28wtOV9LRzWxE2jtyA/qQ2rc=";
        };
        x86_64-darwin = {
          asset = "ai9s_Darwin_amd64.tar.gz";
          hash = "sha256-yY2Beqnsy0ivlY4XH05wxwYU5C2JIcnxaOeoxdnOskI=";
        };
        aarch64-darwin = {
          asset = "ai9s_Darwin_arm64.tar.gz";
          hash = "sha256-ofvuI3JHGVfei2FUoA2B8N2uxG9dVPaJsaKCtvy50xM=";
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
