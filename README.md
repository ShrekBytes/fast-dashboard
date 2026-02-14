# DASH-DASH-DASH

Minimal, fast dashboard: clock, weather, search, bookmarks, to-do, service checks, RSS. Lightweight—runs well as a browser new-tab or home page.

Stripped-down version of [Glance](https://github.com/glanceapp/glance). For more features, use Glance; for less and lightning-fast, this is it.


![DASH-DASH-DASH Preview](quick-start/screenshots/preview.png)


## Table of contents

- [Installation](#installation)
  - [Recommended: Docker Compose](#recommended-docker-compose)
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


## Installation

### Recommended: Docker Compose

1. **Create a project directory** and add config + compose file:

   ```bash
   mkdir -p ~/dash-dash-dash/config
   ```

   - Copy [quick-start/dash-dash-dash/config/config.yml](quick-start/dash-dash-dash/config/config.yml) to `~/dash-dash-dash/config/config.yml`.
   - Copy [quick-start/dash-dash-dash/docker-compose.yml](quick-start/dash-dash-dash/docker-compose.yml) to `~/dash-dash-dash/docker-compose.yml`.
   - (Optional) Create `~/dash-dash-dash/.env` if you need environment variables in config.

2. **Start the stack:**

   ```bash
   cd ~/dash-dash-dash && docker compose up -d
   ```

**Useful commands** (run from `~/dash-dash-dash`):

- **Stop:** `docker compose down`
- **Update image:** `docker compose pull && docker compose up -d`

##

### Docker / Podman Manual

**Create a project directory** and add config (and optionally `.env`):

   ```bash
   mkdir -p ~/dash-dash-dash/config
   ```

   - Copy [quick-start/dash-dash-dash/config/config.yml](quick-start/dash-dash-dash/config/config.yml) to `~/dash-dash-dash/config/config.yml`.
   - (Optional) Create `~/dash-dash-dash/.env` if you need environment variables in config.

**Docker:**

```bash
docker run -d --name dash-dash-dash --restart on-failure \
  --network host \
  -v ~/dash-dash-dash/config:/app/config:Z \
  -v ~/dash-dash-dash/.env:/app/.env:ro \
  ghcr.io/shrekbytes/dash-dash-dash:latest
```

**Podman:** Replace `docker` with `podman` in the command above.

If you don't use `.env`, omit the `-v .../.env:/app/.env:ro` line (or create an empty file).

##

### Podman quadlet

1. **Create the project directory** and add config (and optionally `.env`):

   ```bash
   mkdir -p ~/dash-dash-dash/config
   ```
   Copy [quick-start/dash-dash-dash/config/config.yml](quick-start/dash-dash-dash/config/config.yml) to `~/dash-dash-dash/config/config.yml`. Create `~/dash-dash-dash/.env` only if you need environment variables.

2. **Copy the quadlet file** to the systemd user drop-in directory:

   ```bash
   mkdir -p ~/.config/containers/systemd
   ```
   Copy [quick-start/dash-dash-dash/dash-dash-dash.container](path/to/dash-dash-dash/quick-start/dash-dash-dash/dash-dash-dash.container) to `~/.config/containers/systemd/dash-dash-dash.container`
   
3. **Reload and start:**

   ```bash
   systemctl --user daemon-reload
   systemctl --user start dash-dash-dash.service
   ```

If you don't use `.env`, either create an empty `~/dash-dash-dash/.env` or remove the `EnvironmentFile=` line from the quadlet file.

##

### Run with Go (no Docker)

**Requires Go 1.21+.**

1. Copy a starter config to `config.yml` in your working directory.

2. Build and run:

   ```bash
   go build -o dash-dash-dash .
   ./dash-dash-dash
   ```

   Or with an explicit config path:

   ```bash
   ./dash-dash-dash -config path/to/config.yml
   ```

##

### Usage

Defaults to **http://localhost:8080**. Host and port are set in `config.yml` under `server`.

##

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

##

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

##

## Widgets

Defined under `pages[].columns[].widgets`. Every widget has `type` plus type-specific options. Common to all: `title`, `hide-header`, `css-class`; for data widgets, `cache` (e.g. `1m`, `5m`).

### Clock

| Option | Description |
|--------|-------------|
| `hour-format` | `12h` or `24h` |
| `timezones` | List of `timezone` + `label` |

### Calendar

| Option | Description |
|--------|-------------|
| `first-day-of-week` | `monday` … `sunday` |

### Weather

Uses [Open-Meteo](https://open-meteo.com/) (no API key).

| Option | Description |
|--------|-------------|
| `location` | **Required.** e.g. `"City, Country"` |
| `units` | `metric` or `imperial` |
| `hour-format` | `12h` or `24h` |
| `hide-location`, `show-area-name` | Toggle location/area in label |

### IP address

| Option | Description |
|--------|-------------|
| `public-url` | Omit = ipinfo.io; `""` = hide public IP; or custom URL |
| `interfaces` | e.g. `[wlo1, eth0]` — only show local IP if default route is in list |

### To-do

| Option | Description |
|--------|-------------|
| `id` | **Required.** localStorage key; different ids = separate lists |

### Search

| Option | Description |
|--------|-------------|
| `search-engine` | `duckduckgo`, `google`, `bing`, `perplexity`, `kagi`, `startpage`, or URL with `{QUERY}` |
| `placeholder`, `autofocus`, `new-tab`, `target` | Input and link behavior |
| `bangs` | List of `shortcut`, `title`, `url` (use `{QUERY}` in url) |

### Service monitor

| Option | Description |
|--------|-------------|
| `style` | `""` or `compact` |
| `show-failing-only` | Only show failing sites |
| `sites` | List of: `title`, `url`, `icon`, `same-tab`, `check-url`, `allow-insecure`, `timeout`, `error-url`, `alt-status-codes`, `basic-auth` |

Icons: `si:name`, `di:name`, `mdi:name`, `sh:name`, or full URL. Default timeout 3s.

### Bookmarks

| Option | Description |
|--------|-------------|
| `groups` | List of groups: `title`, `color` (HSL), `same-tab`, `hide-arrow`, `target`, `links` |
| (per link) | `title`, `url`, `icon`, `description`, `same-tab`, `hide-arrow`, `target` |

Icon: empty = favicon; or `si:`, `di:`, `mdi:`, `sh:` + name, or URL.

### RSS

| Option | Description |
|--------|-------------|
| `style` | `list`, `vertical-list`, `detailed-list`, `horizontal-cards`, `horizontal-cards-2` |
| `limit`, `collapse-after` | Max items; show-more after N (`-1` = off) |
| `preserve-order`, `single-line-titles` | Order and title display |
| `feeds` | List of: `url`, `title`, `limit`, `hide-categories`, `hide-description`, `item-link-prefix`, `headers` |

[Caching and refresh](#caching-and-refresh) for cache TTLs. Full example: [config.example.full.yml](quick-start/config.example.full.yml).

##

## Caching and refresh

| Widget | Cache TTL |
|--------|-----------|
| Weather | On the hour |
| Monitor | 5 min |
| RSS | 2 h |
| IP | 10 min |

Background job refreshes due widgets every 5 minutes. Static assets: 24 h; HTML/API: no-cache.

**Health:** `GET /api/healthz` returns 200 when the app is up.

##

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

##

## Troubleshooting

| Issue | What to try |
|-------|-------------|
| Port in use | Change `server.port` in config (e.g. 8081) and restart. With host networking, nothing else should bind that port. |
| Config changes not visible | Edit the file mounted at `/app/config/config.yml` (containers) or passed with `-config` (binary). Save and refresh the page; hot reload picks it up. |
| .env not applied | Restart the container; env is read at start. |
| Config error on start | Run `./dash-dash-dash config:validate`; use `config:print` for the merged config. |

##

## Credits and license

Based on [Glance](https://github.com/glanceapp/glance) by svenstaro. Weather: [Open-Meteo](https://open-meteo.com/). Icons: [DuckDuckGo](https://icons.duckduckgo.com/), [JetBrains Mono](https://www.jetbrains.com/lp/mono/).

**License:** AGPL-3.0. See [LICENSE](LICENSE).
