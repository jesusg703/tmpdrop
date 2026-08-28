# Third-party notices

tmpdrop itself is licensed under the GNU Affero General Public License v3.0; see
[LICENSE](LICENSE).

It has no code dependencies: `go.mod` declares none and the binary is built from
the Go standard library alone. It does embed one third-party asset — the Poppins
typeface — and can talk to one optional external service. Both are covered below
and keep their own licences.

## ConvertX

The optional conversion feature talks to [ConvertX](https://github.com/C4illin/ConvertX)
over HTTP. ConvertX is an independent project licensed under the **AGPL-3.0**.
tmpdrop neither includes nor links its code; the `convertx` profile in
`compose.yaml` pulls its published image and the two containers communicate
over the network. Running that profile means running ConvertX under its own
licence, together with the tools it bundles (ffmpeg, LibreOffice, Calibre,
ImageMagick, Pandoc, libvips and others), each under its own terms. ConvertX is
also AGPL-3.0, but that is coincidental: the two programs only exchange HTTP
requests and neither is linked into the other.

Without the profile, tmpdrop downloads and runs none of that.

## Poppins (typeface)

The interface ships [Poppins](https://github.com/itfoundry/Poppins) as three
WOFF2 files under `internal/server/web/fonts/`, embedded in the binary and
served from `/assets/fonts/`. They are the latin subsets published by Google
Fonts, redistributed unmodified.

Poppins is Copyright 2020 The Poppins Project Authors and is licensed under the
**SIL Open Font License, Version 1.1**, whose full text ships alongside the font
files as `OFL-Poppins.txt`. The OFL explicitly permits embedding and
redistribution, including in commercial work. The OFL and the AGPL are separate
and compatible here: the font is an embedded asset, not linked code, so it keeps
its OFL terms while the program stays AGPL-3.0.
