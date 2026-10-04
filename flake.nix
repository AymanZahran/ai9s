{
  description = "Keyboard-first finder for local AI coding sessions";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-24.11";

  outputs = { self, nixpkgs }:
    let
      version = "1.0.5";
      systems = {
        x86_64-linux = {
          asset = "ai9s_Linux_amd64.tar.gz";
          hash = "sha256-fACjzSJyq8I9ubGVzhSky6bHshkYlDHXQN8a1Rkfcy4=";
        };
        aarch64-linux = {
          asset = "ai9s_Linux_arm64.tar.gz";
          hash = "sha256-VFz14vaJHMaxHME8IA3XATBEoxtzFtlEF91VcSdg5Jg=";
        };
        x86_64-darwin = {
          asset = "ai9s_Darwin_amd64.tar.gz";
          hash = "sha256-aPSjUjz7fV+aAFdv7TSB8z1BcUMHR/SBlAS436VKUco=";
        };
        aarch64-darwin = {
          asset = "ai9s_Darwin_arm64.tar.gz";
          hash = "sha256-Tb4rh36F4fnDl2ghqXLhZuzjkeiAgz/MKAqLFAvYZXg=";
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
