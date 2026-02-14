# Fast Dashboard

A minimal, fast-loading dashboard for browser new-tab or home page. Single binary, no runtime dependencies. Based on [Glance](https://github.com/glanceapp/glance).

![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)
![License](https://img.shields.io/badge/license-AGPL--3.0-blue)

**Requirements:** Go 1.21+ or Docker/Podman. Config: copy `config.example.yml` to `config.yml` (or mount at `/app/config/config.yml` in containers).

---

## Quick start

**Docker (recommended)**

```bash
mkdir -p config && cp config.example.yml config/config.yml
# Edit config/config.yml
docker compose build && docker compose up -d
```

Open **http://localhost:8080**. Set your browser’s new-tab or homepage to this URL.

**From source**

```bash
cp config.example.yml config.yml   # edit as needed
go build -o fast-dashboard .
./fast-dashboard
```

---

## Features

- **Clock** — Local time, 12h/24h, optional timezones
- **Calendar** — Month view, configurable first day of week
- **Weather** — Current + hourly (Open-Meteo, no API key)
- **IP** — Hostname, active local IP (default-route), optional public IP + country
- **Service Monitor** — Health checks with response time and uptime dots (last 10)
- **Search** — URL or search; bangs and custom shortcuts
- **Bookmarks** — Grouped links, favicons or CDN icons (`si:`, `di:`, `mdi:`, `sh:`)
- **RSS** — List, detailed-list, horizontal-cards; per-feed options
- **To-Do** — Client-side (localStorage)
- **Docker Containers** — From Docker/Podman socket, with labels and categories

Single binary, config hot-reload (fsnotify), background refresh every 5 min, health endpoint: `GET /api/healthz`.

---

## Configuration

YAML config path: `-config` flag (default `config.yml`). **config.example.yml** = minimal; **config.example.full.yml** = every option documented.

| Section | Purpose |
|--------|--------|
| `server` | `host`, `port`, `base-url`, optional `assets-path` for `/assets/` |
| `branding` | `app-name`, `footer` (plain text or HTML; empty = hide) |
| `theme` | HSL colors: `background-color`, `primary-color`, `contrast-multiplier`, optional `positive-color`, `negative-color`, `custom-css-file` |
| `pages` | List of pages: `name`, `slug` (empty = `/`), `columns` with `size: small|full` and `widgets` |

**Includes:** `- $include: path/to/file.yml` (paths relative to containing file).  
**Variables:** `${VAR}`, `${secret:name}`, `${readFileFromEnv:VAR}`. Escape literal `$` with `\`.

---

## Widget reference (summary)

| Widget | Key options |
|--------|-------------|
| `clock` | `hour-format: 12h|24h`, `timezones` |
| `calendar` | `first-day-of-week` |
| `weather` | `location`, `units`, `hour-format` |
| `ip-address` | `public-url` (omit = ipinfo.io; `""` = hide), `interfaces` (optional filter) |
| `monitor` | `sites` (url, icon, check-url, timeout, basic-auth, etc.) |
| `search` | `search-engine`, `placeholder`, `bangs` |
| `bookmarks` | `groups` (title, links with url, icon) |
| `rss` | `style`, `feeds`, `limit`, `preserve-order` |
| `to-do` | `id` (localStorage key) |
| `docker-containers` | `sock-path`, `running-only`, `category` (label filter) |

Docker/Podman socket: add your user to the `docker` group (or use Podman socket) so the widget can list containers. See **config.example.yml** for all options.

---

## Deploy

**Docker Compose**

```bash
docker compose build && docker compose up -d
```

Compose mounts `./config` and the Docker socket.

**Podman (Quadlet)**

Copy the Quadlet file to `~/.config/containers/systemd/`, set `Image=` to your image (required for `AutoUpdate=registry`), and ensure config exists at the paths in the file (default: `%h/self-hosted/fast-dashboard/config` and `assets`). Uses `Network=host` — set `server.host` and `server.port` in `config.yml` to match. Optional: `EnvironmentFile` for env vars; uncomment the Docker socket volume for the container widget.

```bash
mkdir -p ~/self-hosted/fast-dashboard/config ~/self-hosted/fast-dashboard/assets
cp config.example.yml ~/self-hosted/fast-dashboard/config/config.yml

cp fast-dashboard.container ~/.config/containers/systemd/
# Edit Image= in the .container file (e.g. ghcr.io/you/fast-dashboard:latest)

systemctl --user daemon-reload
systemctl --user enable --now fast-dashboard
```

**Plain Docker**

```bash
docker build -t fast-dashboard .
docker run -d -p 8080:8080 \
  -v "$(pwd)/config:/app/config:ro" \
  -v /var/run/docker.sock:/var/run/docker.sock:ro \
  fast-dashboard
```

Image expects `config.yml` at `/app/config/config.yml`. Optional: `-v /path/to/assets:/app/assets:ro`.

---

## Commands

| Command | Description |
|---------|-------------|
| `fast-dashboard` | Run server (default config: `config.yml`) |
| `fast-dashboard -config /path/to/config.yml` | Custom config |
| `fast-dashboard config:validate` | Validate config, exit 0 if OK |
| `fast-dashboard config:print` | Print parsed config (includes + variables resolved) |
| `fast-dashboard --version` | Version |
| `fast-dashboard diagnose` | Diagnostics |

Health: **GET /api/healthz** → 200 when up.

---

## Caching

| Widget | TTL |
|--------|-----|
| Weather | On the hour |
| Monitor | 5 min |
| RSS | 2 h |
| IP | 10 min |
| Docker | 1 min |

Background job refreshes due widgets every 5 minutes. Static assets: 24 h cache; HTML/API: no-cache.

---

## Project structure

```
fast-dashboard-go/
├── main.go
├── go.mod, go.sum
├── config.example.yml
├── Dockerfile, docker-compose.yml, fast-dashboard.container
└── internal/fastdashboard/
    ├── main.go, app.go, config.go, cli.go
    ├── widget.go, widget-*.go, theme.go, templates.go
    ├── embed.go (static + templates)
    ├── static/, templates/
```

**How it works:** Server serves the page shell on `/` or `/{slug}`; the browser fetches `/api/pages/{page}/content/`, server refreshes due widgets and returns HTML; a background job refreshes widgets every 5 minutes.

---

## Credits

- [Glance](https://github.com/glanceapp/glance) by svenstaro (AGPL-3.0)
- [Open-Meteo](https://open-meteo.com/), [DuckDuckGo Icons](https://icons.duckduckgo.com/), [JetBrains Mono](https://www.jetbrains.com/lp/mono/)

**License:** AGPL-3.0. See [LICENSE](LICENSE).
