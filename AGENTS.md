# AGENTS.md

## Cursor Cloud specific instructions

RDE is a two-part app built into a single binary: a Go backend (`backend/`, Gin + GORM + embedded SQLite) and a SvelteKit frontend (`frontend/`, pnpm). Standard commands live in the root `Makefile` and `frontend/package.json`; prefer those over reinventing.

### Running the dev environment
- Backend dev server: port `3080`. Frontend dev server (Vite): port `5175`, and it proxies `/api` and `/ws` to `http://localhost:3080` (see `frontend/vite.config.ts`). `make dev` starts both.
- The backend MUST run as root (`sudo`), which is why `make dev` uses `sudo`. File operations run setuid as a Linux user (see below), and privilege elevation only works when the backend process is root.
- To avoid writing to the production data dir (`/var/lib/rde`), run the backend with an explicit data dir, e.g.: `sudo ./rde-backend -p 3080 -data <dir> -db <dir>/rde.db`. Build the binary first with `go build -o rde-backend .` in `backend/`.

### Critical, non-obvious: accounts map to real Linux users
- RDE runs file/desktop operations by setuid-ing to the Linux user whose name matches the logged-in RDE account (`pkg/runas`). If the RDE account name is not an existing OS user, File Manager actions fail with `create executor failed: user not found: <name>`.
- Therefore, when creating the first admin in the setup wizard, use an existing Linux username. On this VM use `ubuntu` (a real OS user). The wizard rejects the reserved names `admin`, `root`, `system`, `administrator`, `guest`.
- Accessing paths the RDE user does not own requires clicking "Access as Administrator" (re-enter the password) to elevate; this only works with the backend running as root.
- The `-init` CLI flag (`rde-backend -init ...`) currently panics with a nil-pointer during admin creation — do not use it. Create the admin via the setup wizard UI or the `/api/v1/setup/*` endpoints (`check/complete` → `user` → `storage/skip` → `network/skip` → `complete`).

### Go toolchain
- The base image ships Go 1.22, but `go.mod` requires Go 1.25.5. With `GOTOOLCHAIN=auto` (the default), any `go` command auto-downloads and re-execs the 1.25.5 toolchain (cached under `~/go/pkg/mod/golang.org/toolchain`). No manual Go install is needed.

### Tests, lint, build
- Backend tests: `cd backend && go test ./...`. Several failures are pre-existing and unrelated to environment setup: `modules/docker` and `modules/files` fail to compile under `go test` (a `go vet` "non-constant format string" finding and a stale `CreateDir` test signature), and `modules/users`, `modules/system` (host disk detection), `modules/sync`, and `core/bootstrap` have pre-existing/environment-dependent assertion failures. The normal `go build` of the backend succeeds.
- Frontend lint: `cd frontend && pnpm lint` (Prettier + ESLint). It currently reports pre-existing Prettier formatting violations in the committed tree, so it exits non-zero even on a clean checkout.
- Frontend build: `pnpm build` (adapter-static SPA, embedded into the Go binary for release). Dev uses `pnpm dev`.

### Optional services
- Docker, aria2, Samba, Syncthing, Ollama, QEMU/KVM, ADB, etc. are all optional; each module logs a warning and disables itself if its tool/daemon is absent. They are not required to run or test the core desktop/file-management flows.
