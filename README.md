# timescarshare-cli

A command-line client for searching [Times Car Share](https://share.timescar.jp)
(タイムズカーシェア) stations.

The website's station search requires a logged-in session, and its login form is
protected by an invisible reCAPTCHA, so the CLI logs in through a real browser
window once and then reuses the resulting session cookies for fast HTTP
requests.

## Features

- `login`: opens a Chrome window, waits for you to sign in (solving the
  reCAPTCHA if prompted), and saves the session cookies
- `search`: station search by address or station name, with pagination and JSON
  output
- `availability`: which cars are free at a station, over a date/time window
- Credentials from `.env` are pre-filled into the login form
- Session cookies are stored outside the repository, in the per-user config
  directory

## Requirements

- Go 1.27 or later
- [mise](https://mise.jdx.dev) (optional, for the `run` and `build` tasks)
- Google Chrome

## Usage

Log in (a Chrome window opens; sign in there):

```bash
mise run run -- login
```

Search stations:

```bash
mise run run -- search 新宿              # by address
mise run run -- search イトシア --by station   # by station name
mise run run -- search 渋谷 --page 3     # later result pages
mise run run -- search 有楽町 --json     # JSON output
```

Show which cars are free at a station (codes come from `search`):

```bash
mise run run -- availability LA10                             # today, now..+12h
mise run run -- availability LA10 --date 2026-10-12 --from 10:00 --to 22:00
mise run run -- availability LA10 BM08 --from 09:00 --to 18:00 --json
```

All times are JST; `--to` earlier than `--from` means the next day.

The session is saved to `session.json` in the per-user config directory
(`~/Library/Application Support/timescarshare/` on macOS). Re-run `login` when
it expires.

### Credentials

Copy `.env.example` to `.env` and fill in your member number and password. mise
loads `.env` automatically, and `login` pre-fills those values into the browser
form so you only have to solve the reCAPTCHA. `.env` is git-ignored.

## Project Structure

```text
.
├── cmd/                # cobra commands (login, search, availability)
├── internal/session/   # browser login and cookie storage
├── internal/timescar/  # station search and availability HTTP client
├── main.go
├── mise.toml
└── go.mod
```

## Release

Versions follow [Semantic Versioning](https://semver.org) as `vX.Y.Z` tags.
Pushing a tag runs `.github/workflows/release.yaml`, which tests and builds with
[GoReleaser](https://goreleaser.com) and publishes archives for Linux, macOS and
Windows (amd64/arm64) with checksums and build provenance attestations to GitHub
Releases. Tag the next version from an up-to-date, clean `main`:

```bash
mise run tag patch --dry-run   # preview: v1.2.3 -> v1.2.4
mise run tag minor             # tag and push after confirmation (M|m|p also work)
```

## Dependencies

- [`github.com/spf13/cobra`](https://github.com/spf13/cobra)
- [`github.com/chromedp/chromedp`](https://github.com/chromedp/chromedp)
- [`github.com/PuerkitoBio/goquery`](https://github.com/PuerkitoBio/goquery)
- [`github.com/lmittmann/tint`](https://github.com/lmittmann/tint)

## License

Add a license that matches how you want to share this project.
