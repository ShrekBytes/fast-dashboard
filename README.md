# DASH-DASH-DASH

Minimal, fast dashboard: clock, weather, search, bookmarks, to-do, service checks, RSS. Lightweight—runs well as a browser new-tab or home page.

Stripped-down version of [Glance](https://github.com/glanceapp/glance). For more features, use Glance; for less and lightning-fast, this is it.

![License](https://img.shields.io/badge/license-AGPL--3.0-blue)

![DASH-DASH-DASH Preview](quick-start/screenshots/preview.png)

---

## Table of contents

- [Installation](#installation)
  - [Recommended: Docker](#recommended-docker)
  - [Docker / Podman by hand](#docker--podman-by-hand)
  - [Podman quadlet](#podman-quadlet)
  - [Run with Go (no Docker)](#run-with-go-no-docker)
- [Usage](#usage)
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

### Recommended: Docker

Fetches the quick-start folder (config + docker-compose) and starts the stack.

```bash
curl -sL https://github.com/ShrekBytes/dash-dash-dash/archive/refs/heads/main.tar.gz | tar xz && cd dash-dash-dash-main/quick-start/dash-dash-dash && docker compose up -d
```

Open **http://localhost:8080**.

- **Stop:** `docker compose down` (from that folder).
- **Update:** `docker compose pull && docker compose up -d`

---

### Docker / Podman by hand

Image: `ghcr.io/shrekbytes/dash-dash-dash:latest`. One folder with `config.yml` (and optional `.env`), mounted at `/app/config`.

**Setup:** Create a folder (e.g. `~/dash-dash-dash`), add `config/config.yml` and optionally `.env`. Copy a starting config from [quick-start/dash-dash-dash/config/config.yml](https://github.com/ShrekBytes/dash-dash-dash/blob/main/quick-start/dash-dash-dash/config/config.yml) or the [full reference](https://github.com/ShrekBytes/dash-dash-dash/blob/main/quick-start/config.example.full.yml).

**Docker:**

```bash
mkdir -p ~/dash-dash-dash/config
# Put config.yml in ~/dash-dash-dash/config, .env in ~/dash-dash-dash
docker run -d --name dash-dash-dash --restart on-failure \
  --network host \
  -v ~/dash-dash-dash/config:/app/config:Z \
  -v ~/dash-dash-dash/.env:/app/.env:ro \
  ghcr.io/shrekbytes/dash-dash-dash:latest
```

**Podman:** Use `podman` instead of `docker` in the command above.

---

### Podman quadlet

Container file: expects data at **`~/dash-dash-dash`** and also requires config/config.yml and .env by default.

Copy the container file to the quadlet directory (e.g. `~/.config/containers/sytemd/dash-dash-dash.service`). Reload the user daemon and start the unit:

```bash
systemctl --user daemon-reload
systemctl --user start dash-dash-dash.service
```


---

### Run with Go (no Docker)

Requires Go 1.21+. Place `config.yml` in the same directory as the binary, or pass `-config`.

Copy [quick-start/config.example.full.yml](https://github.com/ShrekBytes/dash-dash-dash/blob/main/quick-start/config.example.full.yml) as a starting point, then:

```bash
go build -o dash-dash-dash .
./dash-dash-dash
```

With an explicit config path:

```bash
./dash-dash-dash -config path/to/config.yml
```

---

## Usage

Defaults to **http://localhost:8080**. Host and port are set in `config.yml` under `server`.

---

## Configuration

Single YAML config drives everything. Full reference: **[quick-start/config.example.full.yml](https://github.com/ShrekBytes/dash-dash-dash/blob/main/quick-start/config.example.full.yml)**. The Docker curl install uses the minimal example in `quick-start/dash-dash-dash/config/config.yml`.

### Config file location

- **Docker/Podman:** `config.yml` at `/app/config/config.yml` (mount the folder at `/app/config`).
- **Go binary:** Current working directory, or `-config /path/to/config.yml`.

### Includes

Config can be split into multiple files and included:

```yaml
# In the main config:
pages:
  - $include: partials/home-page.yml
```

Paths are relative to the file that contains the `$include`. Recursion limit: 20.

### Hot reload

Config file changes are applied without restart; reload the page. `.env` changes require a restart.

---

## Config reference

### Top-level sections

| Section | Purpose |
|--------|--------|
| `server` | `host`, `port`, `base-url`. Optional: `assets-path`. |
| `document` | Optional `head`: HTML injected into `<head>`. |
| `theme` | `background-color`, `primary-color` (HSL: `"hue sat light"`), `contrast-multiplier`, `text-saturation-multiplier`. Optional: `positive-color`, `negative-color`, `light`, `custom-css-file`. |
| `branding` | `app-name`, `footer` (plain text or HTML; empty = hide). Optional: `logo-text`, `logo-url`, `favicon-url`, `app-icon-url`, `app-background-color`. |
| `pages` | List of pages; each has `name`, `slug` (empty = `/`), `columns` with `size: small \| full` and `widgets`. |

### Pages and columns

- **Page:** `name`, `slug`, `hide-desktop-navigation`, `center-vertically`, `width` (default | wide | slim), optional `head-widgets`.
- **Column:** `size: small` or `full`; each has a `widgets` list.

---

## Widgets

Defined under `pages[].columns[].widgets`. Each has `type` and type-specific options. Common options (where supported): `title`, `hide-header`, `css-class`, `cache` (e.g. `1m`, `5m`, `1h`).

### Clock

`hour-format: 12h | 24h`, `timezones` (list of `timezone` + `label`). Local time plus optional extra timezones.

### Calendar

`first-day-of-week` (e.g. `monday`, `sunday`). Month view.

### Weather

`location` (e.g. `"City, Country"`), `units: metric | imperial`, `hour-format`, `hide-location`, `show-area-name`. Uses Open-Meteo; no API key. Current conditions and hourly forecast.

### IP address

`public-url`: omit = default (ipinfo.io); set to `""` to hide. Optional `interfaces` (e.g. `[wlo1, eth0]`). Shows hostname, local IP, and optionally public IP and country.

### To-do

`id` (localStorage key; different ids = separate lists). Client-side only; data stays in the browser.

### Search

`search-engine` (e.g. `duckduckgo`, `google`, `bing`, or URL with `{QUERY}`), `placeholder`, `autofocus`, `new-tab`, `target`, `bangs` (list of `shortcut`, `title`, `url` with `{QUERY}`). Single bar: URL or search; bangs (e.g. `!yt`) open custom URLs.

### Service monitor

`title`, `style: "" | compact`, `show-failing-only`, `sites` (list). Each site: `title`, `url`, `icon` (e.g. `si:docker` or full URL), `same-tab`, optional `check-url`, `allow-insecure`, `timeout`, `error-url`, `alt-status-codes`, `basic-auth`. Health checks with response time and uptime dots (last 10). Default timeout 3s.

### Bookmarks

`title`, `groups`. Each group: `title`, optional `color` (HSL), `same-tab`, `links`. Each link: `title`, `url`, optional `icon` (empty = favicon; or `si:name`, URL), `description`, `same-tab`, `target`. Icons: `si:`, `di:`, `mdi:`, `sh:` or full URL.

### RSS

`title`, `style: list | vertical-list | detailed-list | horizontal-cards | horizontal-cards-2`, `limit`, `collapse-after` (-1 = off), `preserve-order`, `single-line-titles`, optional `thumbnail-height`, `card-height`, `feeds`. Each feed: `url`, optional `title`, `limit`, `hide-categories`, `hide-description`, `item-link-prefix`, `headers`. See [Caching and refresh](#caching-and-refresh).

Full options and examples: **[quick-start/config.example.full.yml](https://github.com/ShrekBytes/dash-dash-dash/blob/main/quick-start/config.example.full.yml)**.

---

## Caching and refresh

| Widget | Cache TTL |
|--------|-----------|
| Weather | On the hour |
| Monitor | 5 min |
| RSS | 2 h |
| IP | 10 min |

Background job refreshes due widgets every 5 minutes. Static assets: 24 h; HTML/API: no-cache.

**Health:** `GET /api/healthz` returns 200 when the app is up.

---

## CLI

| Command / flag | Description |
|----------------|-------------|
| `-config <path>` | Config file (default: `config.yml`). |
| `--version`, `-v`, `version` | Print version and exit. |
| `config:validate` | Validate config and exit. |
| `config:print` | Print merged config with includes. |
| `diagnose` | Run diagnostics. |

```bash
./dash-dash-dash -config /etc/dash-dash-dash/config.yml
./dash-dash-dash config:validate
./dash-dash-dash config:print
```

---

## Troubleshooting

| Issue | What to try |
|-------|-------------|
| Port in use | Change `server.port` in config (e.g. 8081) and restart. With host networking, nothing else should bind that port. |
| Config changes not visible | Edit the file mounted at `/app/config/config.yml` (containers) or passed with `-config` (binary). Save and refresh the page; hot reload picks it up. |
| .env not applied | Restart the container; env is read at start. |
| Config error on start | Run `./dash-dash-dash config:validate`; use `config:print` for the merged config. |

---

## Credits and license

Based on [Glance](https://github.com/glanceapp/glance) by svenstaro. Weather: [Open-Meteo](https://open-meteo.com/). Icons: [DuckDuckGo](https://icons.duckduckgo.com/), [JetBrains Mono](https://www.jetbrains.com/lp/mono/).

**License:** AGPL-3.0. See [LICENSE](LICENSE).
