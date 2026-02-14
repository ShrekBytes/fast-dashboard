# DASH-DASH-DASH

Minimal, fast dashboard: clock, weather, search, bookmarks, to-do, service checks, RSS. Lightweight—runs well as a browser new-tab or home page.

Stripped-down version of [Glance](https://github.com/glanceapp/glance). For more features, use Glance; for less and lightning-fast, this is it.


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

Widgets live under `pages[].columns[].widgets`. Each widget has a **`type`** and type-specific options.

| Widget type   | Description |
|---------------|-------------|
| `clock`       | Local time and optional extra timezones |
| `calendar`    | Month calendar view |
| `weather`     | Current conditions and hourly forecast (Open-Meteo, no API key) |
| `ip-address`  | Hostname, local IP, optional public IP and country |
| `to-do`       | Client-side task list (localStorage) |
| `search`      | Search bar and URL bar; bangs for custom shortcuts |
| `monitor`     | Service health checks with response time and uptime history |
| `bookmarks`   | Groups of links with optional icons and colors |
| `rss`         | Aggregated feeds with multiple layout styles |

**Common options** (all widgets):

| Option | Description |
|--------|-------------|
| `title` | Widget heading (overrides default) |
| `hide-header` | Hide the widget title bar |
| `css-class` | Extra CSS class on the widget container |
| `cache` | Override refresh interval for data widgets (e.g. `1m`, `5m`, `1h`) |

---

### Clock

| Option | Description |
|--------|-------------|
| `hour-format` | `12h` or `24h` (default: `24h`) |
| `timezones` | List of `timezone` + `label` (e.g. `America/New_York`, `Europe/London`) |

---

### Calendar

| Option | Description |
|--------|-------------|
| `first-day-of-week` | `monday`, `tuesday`, … `sunday` (default: `monday`) |

---

### Weather

| Option | Description |
|--------|-------------|
| `location` | **Required.** Place name, e.g. `"City, Country"` or `"City, Area, Country"` ([Open-Meteo](https://open-meteo.com/), no API key) |
| `units` | `metric` or `imperial` (default: `metric`) |
| `hour-format` | `12h` or `24h` |
| `hide-location` | Hide the location label (default: `false`) |
| `show-area-name` | Show administrative area in label (default: `false`) |

---

### IP address

| Option | Description |
|--------|-------------|
| `public-url` | Omit = use default (ipinfo.io). Set to `""` to hide public IP. Set to a URL for a custom endpoint (plain-text IP; country from second request if needed). |
| `interfaces` | Optional list (e.g. `[wlo1, eth0]`). If set, only show local IP when the default route interface is in this list. |

---

### To-do

| Option | Description |
|--------|-------------|
| `id` | **Required.** localStorage key. Use different `id` values for separate lists. Data stays in the browser. |

---

### Search

| Option | Description |
|--------|-------------|
| `search-engine` | `duckduckgo`, `google`, `bing`, `perplexity`, `kagi`, `startpage`, or a URL containing `{QUERY}` (default: `duckduckgo`) |
| `placeholder` | Input placeholder text (default: "Search or enter URL…") |
| `autofocus` | Focus the input on load (default: `false`) |
| `new-tab` | Open results in a new tab (default: `false`) |
| `target` | Link target, e.g. `_blank` (default: `_blank` when `new-tab` is used) |
| `bangs` | List of `shortcut`, `title`, `url`. Use `{QUERY}` in `url`; bangs (e.g. `!yt query`) open the URL with query substituted. |

---

### Service monitor

| Option | Description |
|--------|-------------|
| `title` | Widget title (default: "Monitor") |
| `style` | `""` (default) or `compact` |
| `show-failing-only` | When `true`, only show sites that are currently failing (default: `false`) |
| `sites` | List of sites (see table below) |

**Per site:**

| Option | Description |
|--------|-------------|
| `title` | Display name |
| `url` | Page URL (and default health-check URL) |
| `icon` | Empty = no icon; `si:name`, `di:name`, `mdi:name`, `sh:name`, or full URL (e.g. `si:docker`) |
| `same-tab` | Open link in same tab (default: `false`) |
| `check-url` | URL used for health check (default: `url`) |
| `allow-insecure` | Skip TLS verification for the check |
| `timeout` | Check timeout, e.g. `5s`, `2m` (default: `3s`) |
| `error-url` | Link when the site is down (e.g. status page) |
| `alt-status-codes` | HTTP codes to treat as success (e.g. `[301, 302]`) |
| `basic-auth` | `username` and `password` for the check (supports `secret:name`) |

---

### Bookmarks

| Option | Description |
|--------|-------------|
| `title` | Widget title (default: "Bookmarks") |
| `groups` | List of groups (see tables below) |

**Per group:**

| Option | Description |
|--------|-------------|
| `title` | Group heading |
| `color` | Optional HSL (e.g. `"200 60 50"`) |
| `same-tab` | Open links in same tab (default: `false`) |
| `hide-arrow` | Hide arrow on links |
| `target` | Link target (e.g. `_blank`) |
| `links` | List of links (see table below) |

**Per link:**

| Option | Description |
|--------|-------------|
| `title` | Link label |
| `url` | Link URL |
| `icon` | Empty = favicon; or `si:name`, `di:name`, `mdi:name`, `sh:name`, or full URL |
| `description` | Optional description |
| `same-tab` | Override group `same-tab` |
| `hide-arrow` | Override group `hide-arrow` |
| `target` | Override group `target` |

---

### RSS

| Option | Description |
|--------|-------------|
| `title` | Widget title (default: "RSS Feed") |
| `style` | `list`, `vertical-list`, `detailed-list`, `horizontal-cards`, `horizontal-cards-2` (default: `list`) |
| `limit` | Max items shown (default: `25`) |
| `collapse-after` | Show "Show more" after N items; use `-1` to disable (default: `5`) |
| `preserve-order` | Keep feed order instead of sorting by newest (default: `false`) |
| `single-line-titles` | Force single-line item titles (default: `false`) |
| `thumbnail-height` | Optional height for thumbnails |
| `card-height` | Optional height for cards (card styles) |
| `feeds` | List of feeds (see table below) |

**Per feed:**

| Option | Description |
|--------|-------------|
| `url` | Feed URL |
| `title` | Optional override for feed name |
| `limit` | Per-feed item limit; `0` = use widget `limit` |
| `hide-categories` | Hide item categories |
| `hide-description` | Hide item description |
| `item-link-prefix` | Prefix for item links |
| `headers` | Optional HTTP headers (e.g. for auth) |

See [Caching and refresh](#caching-and-refresh) for RSS cache TTL.

---

**Full YAML examples:** [quick-start/config.example.full.yml](quick-start/config.example.full.yml)

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
