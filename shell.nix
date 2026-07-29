{ pkgs ? import <nixpkgs> {
    overlays = [
      (final: prev:
        let
          nixpkgs-25-05 = builtins.fetchTarball {
            url = "https://github.com/NixOS/nixpkgs/archive/nixos-25.05.tar.gz";
            sha256 = "sha256-F8WmEwFoHsnix7rt290R0rFXNJiMbClMZyIC/e+HYf0=";
          };
          pkgs-25-05 = import nixpkgs-25-05 { inherit (prev.stdenv.hostPlatform) system; };
        in
        {
          webkitgtk_4_0 = pkgs-25-05.webkitgtk_4_0;
          libsoup_2_4 = pkgs-25-05.libsoup_2_4.overrideAttrs (old: {
            meta = (old.meta or {}) // { knownVulnerabilities = []; };
          });
        }
      )
    ];
  }
}:

(pkgs.buildFHSEnv {
  name = "golc-dev-fhs";
  targetPkgs = pkgs: (with pkgs; [
    go_1_26
    gcc
    pkg-config
    gtk3 gtk3.dev
    webkitgtk_4_0 webkitgtk_4_0.dev
    glib glib.dev
    libsoup_2_4 libsoup_2_4.dev
    glib-networking
    pango pango.dev
    gdk-pixbuf gdk-pixbuf.dev
    cairo cairo.dev
    harfbuzz harfbuzz.dev
    at-spi2-core at-spi2-core.dev
    libepoxy libepoxy.dev
    zlib zlib.dev
    fontconfig fontconfig.dev
    alsa-lib alsa-lib.dev
    rtmidi
    openssl openssl.dev
    libx11 libx11.dev
    libxext libxext.dev
    libxi libxi.dev
    libxtst
    libxrandr libxrandr.dev
    libxinerama libxinerama.dev
    xorgproto
  ]);

  runScript = "bash";

  profile = ''
    export PATH="$HOME/go/bin:$(go env GOPATH 2>/dev/null)/bin:$PATH"

    if [ -x ".tools/toolchains/mage/1.17.2/linux-amd64/mage" ]; then
      export PATH="$PWD/.tools/toolchains/mage/1.17.2/linux-amd64:$PATH"
    fi
    if [ -x ".tools/toolchains/go/1.26.5/linux-amd64/go/bin/go" ]; then
      export PATH="$PWD/.tools/toolchains/go/1.26.5/linux-amd64/go/bin:$PATH"
    fi

    export PKG_CONFIG_PATH="/usr/lib/pkgconfig:/usr/share/pkgconfig"
    export CGO_LDFLAGS="-lfontconfig"
    export WEBKIT_DISABLE_COMPOSITING_MODE=1
  '';
}).env
