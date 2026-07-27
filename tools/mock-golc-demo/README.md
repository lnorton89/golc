# mock-golc-demo

Drives [bonzupii/mock-golc](https://github.com/bonzupii/mock-golc)'s default
Art-Net test rig directly from GOLC's own fixture/deployment/playback/artnet
packages, as an end-to-end check that a patched show actually produces the
DMX bytes the mock rig expects.

## What it patches

Matches mock-golc's default rig (its README's "Default Rig (Universe 0)"
table) exactly:

| Address | Name    | Fixture (`fixtures/*.yaml`) | Mode |
|---------|---------|------------------------------|------|
| 1       | Par 1   | `rgb-par.yaml`                | 4ch  |
| 5       | Par 2   | `rgb-par.yaml`                | 4ch  |
| 9       | Par 3   | `rgb-par.yaml`                | 4ch  |
| 13      | Par 4   | `rgb-par.yaml`                | 4ch  |
| 17      | Wash 1  | `rgbw-wash.yaml`              | 5ch  |
| 22      | Wash 2  | `rgbw-wash.yaml`              | 5ch  |
| 27      | Mover 1 | `moving-head.yaml`            | 9ch  |
| 36      | Mover 2 | `moving-head.yaml`            | 9ch  |

The RGB Par and RGBW Wash fixtures use the `color_red`/`color_green`/
`color_blue`/`color_white` capability types (see
`internal/fixture/model.go`) — independent per-channel color mixing,
distinct from the pre-existing single `color` capability used for discrete
wheel/gel selection (as on the Moving Head's Color Wheel channel here).

## Usage

Build and run mock-golc, then in this repo:

```
go run ./tools/mock-golc-demo -target 127.0.0.1:6454
```

Flags: `-target host:port` (default `127.0.0.1:6454`), `-hz N` (animation
rate, default 30), `-universe N` (default 0, matching mock-golc's default
rig universe).

The demo animates all eight fixtures directly via `playback.Frame` +
`artnet.Encode` + `artnet.EncodeArtDMX` — the same encode path GOLC's real
Art-Net output worker uses — rather than authoring a show file, so it
exercises the identical wire format a live show would produce.
