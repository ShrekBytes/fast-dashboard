# DASH-DASH-DASH

A minimal, fast dashboard for your browser new-tab or home page. Clock, weather, search, bookmarks, to-do, service checks, RSS—whatever you want. Single binary or one container, no fuss.

![License](https://img.shields.io/badge/license-AGPL--3.0-blue)

![DASH-DASH-DASH Preview](quick-start/screenshots/preview.png) 

---

## Table of contents

- [Installation](#installation)
  - [Recommended: curl + Docker](#recommended-curl--docker)
  - [Docker / Podman by hand](#docker--podman-by-hand)
  - [Podman quadlet](#podman-quadlet)
  - [Run with Go (no Docker)](#run-with-go-no-docker)
- [Quick start](#quick-start)
- [Configuration](#configuration)
  - [Config file location](#config-file-location)
  - [Includes](#includes)
  - [Hot reload](#hot-reload)
- [Config reference](#config-reference)
  - [Top-level sections](#top-level-sections)
  - [Pages and columns](#pages-and-columns)
- [Widgets](#widgets)
  - [Clock](#clock)
  - [Calendar](#calendar)
  - [Weather](#weather)
  - [IP address](#ip-address)
  - [To-do](#to-do)
  - [Search](#search)
  - [Service monitor](#service-monitor)
  - [Bookmarks](#bookmarks)
  - [RSS](#rss)
- [Caching and refresh](#caching-and-refresh)
- [CLI](#cli)
- [Troubleshooting](#troubleshooting)
- [Credits and license](#credits-and-license)

---

## Installation

### Recommended: curl + Docker

Download the ready-to-use folder (config + `.env` + `docker-compose.yml`), then start with Docker. One copy-paste.

```bash
curl -sL https://github.com/ShrekBytes/dash-dash-dash/archive/refs/heads/main.tar.gz | tar xz && cd dash-dash-dash-main/quick-start/dash-dash-dash && docker compose up -d
```

You get a folder with `config/`, `.env`, and `docker-compose.yml`. The app runs in the background.

- **First time:** Open **http://localhost:8080**. Set it as your new-tab or homepage if you like.
- **Stop:** In that folder, run `docker compose down`.
- **Update:** `docker compose pull && docker compose up -d`

---

### Docker / Podman by hand

Use the image `ghcr.io/shrekbytes/dash-dash-dash:latest`. You need a folder with `config.yml` (and optionally `.env`). Mount that folder at `/app/config` and pass the env file if used.

Example (Docker):

```bash
mkdir -p ~/dash-dash-dash/config
# put config.yml in ~/dash-dash-dash/config, .env in ~/dash-dash-dash
docker run -d --name dash-dash-dash --restart on-failure \
  --network host \
  -v ~/dash-dash-dash/config:/app/config:Z \
  -v ~/dash-dash-dash/.env:/app/.env:ro \
  ghcr.io/shrekbytes/dash-dash-dash:latest
```

The app reads `config.yml` from `/app/config/config.yml` inside the container.

---

### Podman quadlet

The repo includes `quick-start/dash-dash-dash/dash-dash-dash.container` for Podman quadlet. It expects the dashboard data at **`~/dash-dash-dash`**: that folder should contain `.env`, `config/`, and optionally `assets/`.

1. Copy or symlink the `quick-start/dash-dash-dash` folder to `~/dash-dash-dash`.
2. Copy the container file to your quadlet directory, e.g. `~/.config/containers/containers/dash-dash-dash.container`.
3. Run `podman generate systemd --new --name dash-dash-dash` or let your system manage the unit; the container file uses `AutoUpdate=registry`.

---

### Run with Go (no Docker)

You need Go 1.21+. Put a `config.yml` in the same directory (e.g. copy [quick-start/config.example.full.yml](https://github.com/ShrekBytes/dash-dash-dash/blob/main/quick-start/config.example.full.yml) and edit), then:

```bash
go build -o dash-dash-dash .
./dash-dash-dash
```

Or specify a config file:

```bash
go build -o dash-dash-dash . && ./dash-dash-dash -config config.example.yml
```

---

## Quick start

After installation, open **http://localhost:8080** (or the host/port you set in `server.port` and `server.base-url`). Set your browser’s new-tab or homepage to this URL to use the dashboard as your start page.

---

## Configuration

All behavior is driven by a single YAML config file. Full reference (every section and widget option documented): **[quick-start/config.example.full.yml](https://github.com/ShrekBytes/dash-dash-dash/blob/main/quick-start/config.example.full.yml)** — copy and trim to your needs. The curl install uses a minimal example in `quick-start/dash-dash-dash/config/config.yml`.

### Config file location

- **Docker/Podman:** The app expects `config.yml` at `/app/config/config.yml` (mount your folder at `/app/config`).
- **Go binary:** By default the binary looks for `config.yml` in the current working directory. Override with `-config /path/to/config.yml`.

### Includes

You can split config into multiple files and include them:

```yaml
# In your main config.yml:
pages:
  - $include: partials/home-page.yml
```

Paths are relative to the file that contains the `$include`. Recursion limit is 20.

### Hot reload

When you change the config file on disk, the app reloads it automatically (fsnotify). Refresh the browser to see changes; no need to restart the container or binary for most edits. Changes to `.env` require a restart.

---

## Config reference

### Top-level sections

| Section | Purpose |
|--------|--------|
| `server` | `host`, `port`, `base-url` (used for links and assets). Optional: `assets-path`. |
| `document` | Optional `head`: HTML injected into `<head>`. |
| `theme` | `background-color`, `primary-color` (HSL: `"hue sat light"`), `contrast-multiplier`, `text-saturation-multiplier`. Optional: `positive-color`, `negative-color`, `light`, `custom-css-file`. |
| `branding` | `app-name`, `footer` (plain text or HTML; empty = hide). Optional: `logo-text`, `logo-url`, `favicon-url`, `app-icon-url`, `app-background-color`. |
| `pages` | List of pages; each has `name`, `slug` (empty = `/`), `columns` with `size: small \| full` and `widgets`. |

### Pages and columns

- **Page:** `name` (shown in nav), `slug` (URL path; empty = root `/`), `hide-desktop-navigation`, `center-vertically`, `width` (default \| wide \| slim), optional `head-widgets`.
- **Column:** `size: small` or `full`. Each column has a `widgets` list.

---

## Widgets

Widgets are listed under `pages[].columns[].widgets`. Each widget has `type` and type-specific options. Common options (when supported): `title`, `hide-header`, `css-class`, `cache` (override TTL, e.g. `1m`, `5m`, `1h`).

### Clock

- **Options:** `hour-format: 12h | 24h`, `timezones` (list of `timezone` + `label`).
- Shows local time and optional extra timezones.

### Calendar

- **Options:** `first-day-of-week` (e.g. `monday`, `sunday`).
- Month view.

### Weather

- **Options:** `location` (e.g. `"City, Country"`), `units: metric | imperial`, `hour-format: 12h | 24h`, `hide-location`, `show-area-name`.
- Uses Open-Meteo; no API key. Current conditions and hourly forecast.

### IP address

- **Options:** `public-url`: omit = use default (ipinfo.io, IP + country); set to `""` to hide public IP. Optional `interfaces` (e.g. `[wlo1, eth0]`) to limit which interfaces are shown.
- Shows hostname, local IP (default route), and optionally public IP and country.

### To-do

- **Options:** `id` (localStorage key; use different ids for separate lists).
- Client-side only; data stays in your browser.

### Search

- **Options:** `search-engine` (e.g. `duckduckgo`, `google`, `bing`, or a URL with `{QUERY}`), `placeholder`, `autofocus`, `new-tab`, `target`, `bangs` (list of `shortcut`, `title`, `url` with `{QUERY}`).
- Single bar: type a URL or search; bangs (e.g. `!yt`) open custom URLs.

### Service monitor

- **Options:** `title`, `style: "" | compact`, `show-failing-only`, `sites` (list). Each site: `title`, `url`, `icon` (e.g. `si:docker`, or full URL), `same-tab`, optional `check-url`, `allow-insecure`, `timeout`, `error-url`, `alt-status-codes`, `basic-auth` (username/password; password can use `secret:name`).
- Health checks with response time and uptime dots (last 10). Default check URL = `url`; default timeout 3s.

### Bookmarks

- **Options:** `title`, `groups`. Each group: `title`, optional `color` (HSL), `same-tab`, `links`. Each link: `title`, `url`, optional `icon` (empty = favicon; or `si:name`, URL), `description`, `same-tab`, `target`.
- Icons: use `si:`, `di:`, `mdi:`, `sh:` for Simple Icons, Devicons, Material Design Icons, or a full URL.

### RSS

- **Options:** `title`, `style: list | vertical-list | detailed-list | horizontal-cards | horizontal-cards-2`, `limit`, `collapse-after` (-1 to disable), `preserve-order`, `single-line-titles`, optional `thumbnail-height`, `card-height`, `feeds`. Each feed: `url`, optional `title`, `limit` (0 = use widget limit), `hide-categories`, `hide-description`, `item-link-prefix`, `headers`.
- Fetched and cached; see [Caching and refresh](#caching-and-refresh).

For every option and example, see **[quick-start/config.example.full.yml](https://github.com/ShrekBytes/dash-dash-dash/blob/main/quick-start/config.example.full.yml)**.

---

## Caching and refresh

| Widget | Cache TTL |
|--------|-----------|
| Weather | On the hour |
| Monitor | 5 min |
| RSS | 2 h |
| IP | 10 min |

A background job refreshes widgets that are due every 5 minutes. Static assets are cached 24 h; HTML/API responses are no-cache.

**Health endpoint:** `GET /api/healthz` returns 200 when the app is up. Useful for reverse proxies or monitoring.

---

## CLI

When running the binary (Go):

| Command / flag | Description |
|----------------|-------------|
| `-config <path>` | Config file path (default: `config.yml`). |
| `--version`, `-v`, `version` | Print version and exit. |
| `config:validate` | Validate the config file and exit. |
| `config:print` | Print the parsed config with includes resolved. |
| `diagnose` | Run diagnostic checks. |

Example:

```bash
./dash-dash-dash -config /etc/dash-dash-dash/config.yml
./dash-dash-dash config:validate
./dash-dash-dash config:print
```

---

## Troubleshooting

| Issue | What to try |
|-------|-------------|
| Port already in use | Change `server.port` in config (e.g. to 8081) and restart. With Docker host networking, nothing else should use that port on the host. |
| Config changes not visible | Ensure you’re editing the file mounted at `/app/config/config.yml` (containers) or the file passed with `-config` (binary). Save the file and refresh the browser; hot reload should pick it up. |
| .env not applied | Restart the container after changing `.env`; env is read at start. |
| Config error on start | Run `./dash-dash-dash config:validate` (or the binary with `config:validate`) to see validation errors. Use `config:print` to see the merged config. |

---

## Credits and license

Based on [Glance](https://github.com/glanceapp/glance) by svenstaro. Weather via [Open-Meteo](https://open-meteo.com/). Icons: [DuckDuckGo](https://icons.duckduckgo.com/), [JetBrains Mono](https://www.jetbrains.com/lp/mono/).

**License:** AGPL-3.0. See [LICENSE](LICENSE).
