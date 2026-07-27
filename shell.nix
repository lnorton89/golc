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
          # Wails v2 requires webkitgtk_4_0 (WebKitGTK 2.44.x), which was
          # removed from nixpkgs after 25.05. Pull it from the last
          # release that shipped it. webkitgtk_4_0 was built against
          # libsoup 2.x, which is also marked insecure on the host
          # channel, so pull that from 25.05 too.
          webkitgtk_4_0 = pkgs-25-05.webkitgtk_4_0;
          # Strip vulnerability flags since this is a dev/build-only
          # dependency, not shipped in the GOLC product.
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

  # Only packages the project's own toolchain (mage Bootstrap) cannot
  # provision itself: system C libraries that Wails, GTK, WebKit, ALSA,
  # and MIDI link against, plus the minimum seed build tools.
  #
  # NOTE: Nix multi-output derivations split `out` (shared libraries)
  # from `dev` (headers + .pc files). buildFHSEnv only links the
  # directories of the derivations listed here, so for CGo/pkg-config to
  # find headers and .pc files the `*.dev` outputs must be listed
  # explicitly -- listing `gtk3` alone links the .so files but NOT the
  # headers or `gtk+-3.0.pc`.
  targetPkgs = pkgs: (with pkgs; [
    go_1_26

    # C build essentials
    gcc
    pkg-config

    # Wails v2 Linux runtime / compile-time dependencies
    # .dev outputs provide the headers + .pc files CGo needs.
    # buildFHSEnv does NOT propagate transitive build inputs, so every
    # .pc file that gtk3 / webkitgtk / gdk-3.0 may require at compile
    # time must be listed here explicitly.
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

    # MIDI / audio
    alsa-lib alsa-lib.dev
    rtmidi

    # TLS (Go crypto may link against system OpenSSL on some platforms)
    openssl openssl.dev

    # X11 (golang.design/x/hotkey links -lX11 and #includes <X11/Xlib.h>,
    # and Wails' webkit2gtk binding reach X11 transitively). Most of
    # these are multi-output (out=.so, dev=headers+.pc) on this channel;
    # libxtst is single-output and ships headers+.pc in `out`.
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
    # Prevent Go 1.21+ from adding a 'toolchain' directive to go.mod
    # during go mod download (bootstrap rejects any lock-file mutation).
    # export GOTOOLCHAIN="local"

    # Standard Go user binary paths (preserved from original shell.nix)
    export PATH="$HOME/go/bin:$(go env GOPATH 2>/dev/null)/bin:$PATH"

    # After 'mage Bootstrap' completes, prefer the project-local
    # bootstrapped toolchain over whatever is on the host PATH.
    if [ -x ".tools/toolchains/mage/1.17.2/linux-amd64/mage" ]; then
      export PATH="$PWD/.tools/toolchains/mage/1.17.2/linux-amd64:$PATH"
    fi
    if [ -x ".tools/toolchains/go/1.26.5/linux-amd64/go/bin/go" ]; then
      export PATH="$PWD/.tools/toolchains/go/1.26.5/linux-amd64/go/bin:$PATH"
    fi

    # buildFHSEnv links .pc files from the *.dev outputs into here.
    export PKG_CONFIG_PATH="/usr/lib/pkgconfig:/usr/share/pkgconfig"
    # pangocairo.pc has fontconfig in Requires.private, so pkg-config
    # --libs (without --static) omits -lfontconfig. Force the link.
    export CGO_LDFLAGS="-lfontconfig"
    export WEBKIT_DISABLE_COMPOSITING_MODE=1
  '';
}).env
