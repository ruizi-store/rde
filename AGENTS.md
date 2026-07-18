# Agent Guide — RDE Development Environment

RDE (Ruizi Desktop Environment) is a web-based Linux desktop with a **Go** backend and **SvelteKit** frontend.

## Prerequisites

| Tool | Required version |
|------|------------------|
| Go | 1.25.5+ (see `backend/go.mod`) |
| Node.js | 20.19.1 (Makefile default) |
| pnpm | latest |

System packages: `sudo` (backend runs as root via `make dev`), `curl`, `make`.

## One-time setup

```bash
# Install / upgrade Go, Node, pnpm
make setup
export PATH=/usr/local/go/bin:/usr/local/node/bin:$PATH

# Runtime directories (required for backend defaults)
sudo mkdir -p /var/lib/rde/db /var/lib/rde/conf /var/log/rde /var/run/rde /etc/rde
sudo cp -n backend/conf/conf.conf.sample /etc/rde/rde.conf
sudo chmod -R a+rwX /var/lib/rde /var/log/rde /var/run/rde

# Dependencies
cd frontend && pnpm install --config.dangerouslyAllowAllBuilds=true && cd ..
cd backend && go mod download && cd ..
```

Notes:

- `make setup` installs Go/Node under `/usr/local`. Prefer that PATH over system Go/Node.
- If Chinese registries are unreachable, use:
  - `go env -w GOPROXY=https://proxy.golang.org,direct`
  - `npm config set registry https://registry.npmjs.org`
- pnpm 10 may skip dependency build scripts; allow builds for `esbuild` (Vite) as above.

## Run

```bash
make dev     # backend :3080, frontend :5175
make stop    # stop both
```

Or separately:

```bash
# Backend (needs write access to /var/lib/rde; make dev uses sudo)
cd backend && sudo ./rde-backend   # after: go build -o rde-backend .

# Frontend (proxies /api and /ws to :3080)
cd frontend && pnpm dev --port 5175
```

## Verify

- Frontend UI: `http://localhost:5175/` (setup wizard when uninitialized)
- Backend health: `curl -s http://localhost:3080/health`
- Setup API: `curl -s http://localhost:3080/api/v1/setup/status`
- Proxy via Vite: `curl -s http://localhost:5175/api/v1/setup/status`

On first run, setup is incomplete (`completed: false`) until the wizard creates an admin user.

## Layout

```
backend/     Go API (Gin), modules under backend/modules/
frontend/    SvelteKit + Tailwind v4 + Vite
debian/      DEB packaging
Makefile     setup / dev / stop / deb
```

## Common pitfalls

1. **Wrong Go version** — system Go (e.g. 1.22) cannot build this repo; use `/usr/local/go/bin/go` after `make setup`.
2. **Missing data dirs** — backend defaults to `/var/lib/rde` and `/etc/rde/rde.conf`.
3. **Port mismatch** — README mentions `:80` / `:5173` for production-ish flows; `make dev` uses **3080** / **5175**.
4. **Optional services** — Docker, Flatpak, LibreTranslate, aria2, etc. may log warnings when absent; core UI/API still works.
