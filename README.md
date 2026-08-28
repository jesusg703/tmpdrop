# tmpdrop

A self-hosted, temporary file sharing service. Upload a file, share the link,
and it disappears when the timer runs out.

Written in Go with **only the standard library**. A single static binary, an
embedded web UI, a JSON manifest for metadata and local disk for storage.

## Features

- HTTP upload and download of files stored locally.
- Per-file lifetime (default TTL) with automatic expiration and a background
  sweeper.
- Configurable limits: maximum file size, total storage, per-client quota and
  per-client file count.
- Optional link expiry by download count (`max_downloads`); the file is deleted
  as soon as the last allowed download is delivered.
- Range requests, so a preview can seek and a cut download resumes. Ranged and
  `HEAD` requests never spend a download.
- Minimalist dark web UI: drag-and-drop upload, a copy button for the share
  link, in-browser preview, convert and delete.
- Conversion to other formats through [ConvertX](https://github.com/C4illin/ConvertX),
  included and working out of the box (the account is created automatically),
  and hidden entirely when you turn it off.
- Logging, panic recovery, security headers and graceful shutdown.
- Unit and integration tests; Makefile with `build`, `test` and `lint`.
- No external assets: the page pulls nothing from the internet at runtime.
- Containerised with a multi-stage Dockerfile; the image runs as an
  unprivileged user on a read-only root filesystem.

## Quick start (Docker)

```bash
git clone https://github.com/jesusg703/tmpdrop.git
cd tmpdrop
docker compose up -d
# open http://localhost:8080, or http://<this-machine>:8080 from anywhere on your network
```

That is the whole setup. No config file, no variables, no reverse proxy:
uploads, share links, expiry, passwords and **format conversion** all work
straight away.

Two things worth knowing before you run it.

**The first start downloads several GB.** Conversion is handled by
[ConvertX](https://github.com/C4illin/ConvertX), which bundles ffmpeg,
LibreOffice and Calibre — that is where the size comes from, and it is what lets
tmpdrop turn a `.docx` into a PDF or an `.mkv` into an MP3. The tmpdrop image
itself is a few MB. If you do not want conversion at all, set `CONVERTX_URL=` (an
empty value) in a `.env` file: the Convert button disappears and everything else
behaves the same.

**There is no login, and the port is open to your network.** Anyone who can
reach it can list every file and delete any of them. That is deliberate — it is a
shared board for a network you trust — but it means you should not put it
somewhere untrusted as it stands. To keep it on the local machine only:

```bash
echo 'TMPDROP_BIND=127.0.0.1' >> .env
docker compose up -d
```

To share it more widely than a LAN, put it behind a reverse proxy that asks for
credentials — see [Behind a reverse proxy](#behind-a-reverse-proxy).

The ConvertX account is **created automatically** on first use, with
`CONVERTX_EMAIL` / `CONVERTX_PASSWORD` (defaults `tmpdrop@example.com` /
`tmpdrop-demo`). Change them in `.env` before the first start if you also plan to
expose ConvertX itself; changing them afterwards does not update the account
already stored in its volume.

## Build from source

Requires Go 1.24.

```bash
make build          # produces bin/tmpdrop
make test           # go test ./...
make lint           # go vet + gofmt check
make run            # run with defaults on :8080

./bin/tmpdrop -version   # print the build version and exit
```

No Go toolchain on the machine? The `-docker` variants run everything in a
container: `make test-docker`, `make build-docker`, `make lint-docker`.

## Configuration

Settings come from three sources, each overriding the previous one:
**built-in defaults** < **config file** (JSON) < **environment variables**.

A sample file is provided as `config.example.json`. Any key can also be set as
an environment variable with the same name (uppercased, `TMPDROP_` prefix for
app settings, `CONVERTX_` for the conversion service), and the environment
wins. The config file is read from `TMPDROP_CONFIG` or `./config.json` if
present.

| config.json key        | Env variable                | Default      | Meaning                              |
|------------------------|-----------------------------|--------------|--------------------------------------|
| `addr`                 | `TMPDROP_ADDR`               | `:8080`      | Listen address                       |
| `storage_dir`          | `TMPDROP_STORAGE_DIR`        | `./data`     | Directory for blobs + manifest       |
| `max_file_size`        | `TMPDROP_MAX_FILE_SIZE`      | `100MB`      | Per-file upload limit                |
| `max_storage`          | `TMPDROP_MAX_STORAGE`        | `1GB`        | Total storage limit                  |
| `default_ttl`          | `TMPDROP_DEFAULT_TTL`        | `24h`        | Default lifetime of uploads (`0` = no expiry) |
| `sweep_interval`       | `TMPDROP_SWEEP_INTERVAL`     | `1m`         | How often expired files are purged   |
| `shutdown_timeout`     | `TMPDROP_SHUTDOWN_TIMEOUT`   | `10s`        | Grace period on shutdown             |
| `quota_default`        | `TMPDROP_QUOTA_DEFAULT`      | `250MB`      | Per-client byte quota (`0` disables) |
| `max_files_per_client` | `TMPDROP_MAX_FILES_PER_CLIENT` | `50`       | Per-client file count (`0` disables) |
| `log_level`            | `TMPDROP_LOG_LEVEL`          | `info`       | `debug`, `info`, `warn`, `error`     |
| `source_url`           | `TMPDROP_SOURCE_URL`         | upstream repo | Source link in the footer (AGPL §13) |
| `trusted_proxies`      | `TMPDROP_TRUSTED_PROXIES`    | *(empty)*    | IPs/CIDRs whose `X-Forwarded-For` is believed |
| `convertx.url`         | `CONVERTX_URL`              | *(empty)*    | Base URL of ConvertX; empty disables it. The compose file sets it |
| `convertx.email`       | `CONVERTX_EMAIL`            | *(empty)*    | Account email (auto-created)         |
| `convertx.password`    | `CONVERTX_PASSWORD`         | *(empty)*    | Account password                     |
| `convertx.timeout`     | `CONVERTX_TIMEOUT`          | `5m`         | Max time for one conversion          |

Sizes accept plain bytes or units like `512KB`, `2MB`, `1GB`. Durations use Go
syntax (`30s`, `1m`, `24h`).

> `convertx.url` requires `convertx.email` and `convertx.password`: with a URL
> set and either of those empty, tmpdrop refuses to start. Emptying `url` is
> what turns the integration off.

There is also `TMPDROP_BIND`, which is not a tmpdrop setting at all — the
compose file uses it to choose the interface the port is published on.

## Behind a reverse proxy

tmpdrop speaks plain HTTP and builds its links from the request, so any proxy
works without configuring a base URL. Three things do need attention.

**Serve it at the root of a name of its own** — `files.example.com`, not
`example.com/tmpdrop/`. The interface asks for `/api/files` and `/d/{id}`
absolutely; there is no subpath support.

**Raise the upload limit.** Defaults are far below tmpdrop's own 100 MB, and the
symptom is a `413` on the first real file.

```nginx
# nginx
location / {
    proxy_pass http://127.0.0.1:8080;
    client_max_body_size 100m;       # default is 1m
    proxy_request_buffering off;     # or every upload lands on the proxy's disk first
    proxy_set_header Host $host;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
}
```

```caddyfile
# Caddy
files.example.com {
    request_body { max_size 100MB }
    reverse_proxy 127.0.0.1:8080
}
```

**List the proxy in `trusted_proxies`** (for example `["127.0.0.1", "10.0.0.0/8"]`).
Without it tmpdrop sees every request as coming from the proxy, so per-client
quotas and the password rate limit end up shared by everyone. The
`X-Forwarded-For` header is ignored unless the request really arrives from a
listed address, so it cannot be forged.

Since the proxy is where authentication belongs, that is also the place to add
it — basic auth, an SSO forward-auth, whatever you already run.

## Usage

### Web UI

`GET /` serves the single-page interface. Upload a file, optionally set an
expiration, a download cap and a password, then copy the share link from the box
that appears. Every row offers copy, download, convert and delete; clicking the
name previews the file in the browser.

The copy button uses the clipboard API where the browser allows it — that means
HTTPS or localhost — and falls back to an older method over plain HTTP, which is
how most self-hosted installs are reached. If both are blocked the link is still
there in the box, selected and ready to copy by hand.

### HTTP API

| Method   | Path                    | Description                                 |
|----------|-------------------------|---------------------------------------------|
| `GET`    | `/healthz`              | Health check (`ok`)                         |
| `POST`   | `/api/upload`           | Multipart upload (field `file`)             |
| `POST`   | `/upload`               | Same, but answers with a redirect (web)     |
| `GET`    | `/api/files`            | List files + storage usage                  |
| `GET`    | `/api/files/{id}`       | Metadata of one file                        |
| `DELETE` | `/api/files/{id}`       | Delete a file                               |
| `GET`    | `/d/{id}`               | Download (`?inline=1` to preview in-browser)        |
| `POST`   | `/api/files/{id}/convert` | Start a conversion, body `{"target":"mp3"}` |
| `GET`    | `/api/convert/{id}`     | Conversion status (`running`/`done`/`error`)|


Range requests are supported. A ranged or `HEAD` request never counts towards
`max_downloads`, so a link preview by a chat client does not spend the link.
Protected files take the password in an `X-File-Password` header; it is not
accepted in the query string, where it would leak into proxy logs.

Upload example:

```bash
curl -F "file=@report.pdf" -F "ttl=168h" -F "note=weekly report" \
     http://localhost:8080/api/upload
```

```json
{
  "id": "a1b2c3d4e5f6a7b8c9d0e1f2",
  "name": "report.pdf",
  "size": 123456,
  "expires_at": "2026-09-01T12:00:00Z",
  "url": "/d/a1b2c3d4e5f6a7b8c9d0e1f2?inline=1",
  "download_url": "/d/a1b2c3d4e5f6a7b8c9d0e1f2",
  "convert_available": true
}
```

### Conversion

`POST /api/files/{id}/convert` with `{"target":"mp3"}` starts a conversion on
the linked ConvertX instance. Poll `GET /api/convert/{id}`; when it returns
`"status":"done"` a new file with its own share link has been stored under the
same lifetime as the source. Supported targets are the common ones listed in
`internal/convertx/formats.go` (audio/video, office, markup, images, e-books,
contacts); extend that map to enable more.

## Storage layout

```
data/
├── manifest.json   # metadata, written atomically (temp + rename)
└── files/          # blobs named by random id (24 hex chars)
```

Original filenames live only in the manifest and are restored on download; they
can never touch the filesystem path.

**tmpdrop has no authentication.** IDs are random, but `GET /api/files` lists
every file with its id, and `DELETE` removes any of them, so anyone who can
reach the port can read and delete everything. It is built as a shared board
for a network you trust; put it behind a reverse proxy with authentication if
that is not your case.

## Interface

Dark by default, one warm accent, and nothing fetched at runtime. Everything is
driven by custom properties in `:root`, so retheming means editing a dozen
values rather than hunting through rules.

| Token | Value | Used for |
|---|---|---|
| `--bg` / `--panel` / `--panel-soft` | `#101010` / `#181818` / `#1e1e1e` | Page, cards, insets. Off-black rather than pure black: it keeps borders visible and is easier on the eyes against light text |
| `--border` / `--border-strong` | `#262626` / `#383838` | Dividers, and the heavier stroke buttons need |
| `--fg` / `--muted` | `#f4f1ea` / `#a09d96` | Warm off-white, because neutral grey fights the gold |
| `--accent` / `--accent-strong` / `--accent-soft` / `--accent-bg` | `#d8b063` / `#bd9139` / `#e0c787` / `#231c10` | Fill, hover, text-on-dark, tinted backgrounds |
| `--danger` | `#e07a7a` | Destructive actions |
| `--radius` / `--radius-sm` | `12px` / `8px` | Cards and inputs |
| `--shadow` | `0 16px 36px rgba(0,0,0,.5)` | What actually makes the cards lift off the page |

Buttons come in three levels and **only one of them is filled**: `.btn.primary`
for the action the page exists for, plain `.btn` for everything else, and
`.btn.danger` as text only — deleting should not shout louder than uploading.

Headings use positive letter-spacing so the product name reads as a mark, and
panel titles are small tracked uppercase, which turns an `<h2>` into a section
label instead of a headline.

The typeface is [Poppins](https://github.com/itfoundry/Poppins), served from the
binary at `/assets/fonts/`. It is **not** loaded from a CDN: the page renders
identically on a machine with no internet access, which is the whole point of a
self-hosted tool. Poppins is licensed under the SIL Open Font License 1.1 — see
[THIRD-PARTY-NOTICES.md](THIRD-PARTY-NOTICES.md). The program itself stays AGPL-3.0.

## Tests

```bash
make test          # or: make test-docker
```

Covers configuration precedence, store behaviour (limits, quotas, expiry,
sweep, persistence), the ConvertX client against a simulated instance
(auto-registration on both 401 and 403, re-auth on invalid sessions, streamed
uploads declaring an exact Content-Length, timeouts) and the full HTTP flow from
upload to converted download.

## Implementation notes

The code has no comments. The decisions that are not obvious from reading it —
why ranged requests do not count as downloads, why some conversion formats are
deliberately not offered, why blob reclamation refuses to run in some cases —
are in [docs/NOTES.md](docs/NOTES.md).

## License

[GNU Affero General Public License v3.0](LICENSE).

AGPL-3.0 is a strong copyleft licence. In short: you may run, study, modify and
redistribute this, but derivative work must stay under the same licence — and
because of **section 13**, if you run a modified version as a network service
you have to offer its source to the people using it.

That last point is why the page footer carries a "Source code" link. **If you
fork and modify tmpdrop, point that link at your own repository**, either with
`source_url` in the config file or `TMPDROP_SOURCE_URL` in the environment.
Leaving it aimed at the upstream project does not satisfy section 13.

The bundled typeface keeps its own licence; see
[THIRD-PARTY-NOTICES.md](THIRD-PARTY-NOTICES.md).

