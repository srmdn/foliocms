> **Archived project (October 2026).** FolioCMS is no longer maintained and
> the hosted demo has been retired. The source remains available read-only for
> reference. The `foliocms.com` domain is available for acquisition; contact
> [mail@srmdn.com](mailto:mail@srmdn.com).

# Folio

A lightweight, self-hostable CMS. Single Go binary. SQLite. Ships with a default Astro theme. No Docker required.

## Why

Most open source CMS options are either too heavy (WordPress, Ghost) or too opinionated about the frontend (Payload, Strapi). Folio is built for developers who self-host on a cheap VPS and want:

- A real Markdown editor out of the box
- A default frontend theme they can swap
- Zero external dependencies: no PostgreSQL, no Redis, no Docker
- One systemd service and done

## Requirements

- Go 1.21+
- Node.js 18+ and npm
- Git
- Linux or macOS

> **Note:** Node.js 22 or later is required for the default theme (Astro 6).

## Install

Download and review the installer, then run it:

```bash
curl -O https://raw.githubusercontent.com/srmdn/foliocms/main/install.sh
bash install.sh
```

This will:

1. Clone Folio and build the binary from source
2. Clone and build the default Astro theme
3. Create a `.env` file with a generated `JWT_SECRET`
4. Run the first-time setup wizard to create your admin account

By default, everything is installed into `./folio/`. To use a different directory:

```bash
bash install.sh --dir /opt/folio
```

## Running

After install:

```bash
cd folio

# Start the backend API (default port 8090)
./folio

# In a separate terminal: start the theme (default port 4321)
cd theme && node dist/server/entry.mjs
```

For production, run both as systemd services. See [docs/configuration.md](docs/configuration.md) for all environment variables.

## First-time setup

The installer runs `--setup` for you. If you need to re-run it later:

```bash
./folio --setup
```

This creates the admin account interactively. You can also set `ADMIN_EMAIL` and `ADMIN_PASSWORD` in `.env` instead.

## Configuration

All configuration is via `.env`. See [docs/configuration.md](docs/configuration.md) for the full reference.

Key variables:

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8090` | Backend API port |
| `JWT_SECRET` | (none) | Required. Generate with `openssl rand -hex 32` |
| `CONTENT_DIR` | `content/blog` | Path to Markdown content |
| `THEME_DIR` | `theme` | Path to the theme directory |
| `THEME_BUILD_CMD` | `npm run build` | Command to rebuild the theme |
| `THEME_SERVICE` | (none) | systemd service name to restart after rebuild |
| `SITE_URL` | `http://localhost:8090` | Base URL used for media file URLs |
| `MEDIA_STORAGE` | `local` | Storage backend: `local` or `s3` |

## Admin Dashboard

Folio includes a built-in admin dashboard at `/admin`. No separate service or install step required — it is embedded in the Go binary.

| Path | Description |
|------|-------------|
| `/admin/login` | Sign in |
| `/admin/posts` | Create, edit, publish, and delete posts |
| `/admin/media` | Upload, browse, and delete images |
| `/admin/subscribers` | View and remove newsletter subscribers |
| `/admin/settings` | Edit site settings, trigger a rebuild, view build status |

The post editor uses [Milkdown](https://milkdown.dev), a WYSIWYG Markdown editor with support for headings, lists, code blocks, tables, and more. Press **Cmd+S** (or **Ctrl+S** on Windows/Linux) to save at any time.

## Media Library

The media library lets you upload images from the dashboard and insert them directly into posts.

By default, images are stored on disk in a `media/` directory next to `CONTENT_DIR` and served at `/media/{key}`. To use S3-compatible object storage instead (AWS S3, Cloudflare R2, NevaObjects, MinIO), set `MEDIA_STORAGE=s3` and the `S3_*` variables in `.env`:

```env
MEDIA_STORAGE=s3
S3_ENDPOINT=https://s3.nevaobjects.id
S3_BUCKET=my-bucket
S3_REGION=auto
S3_ACCESS_KEY=your-access-key
S3_SECRET_KEY=your-secret-key
S3_PUBLIC_URL=https://s3.nevaobjects.id/my-bucket
```

See [docs/configuration.md](docs/configuration.md) for provider-specific endpoint examples.

## Site Settings

Site metadata is stored in the database and can be updated from the dashboard at `/admin/settings` without restarting the server. The public `GET /api/settings` endpoint exposes these values to themes.

| Key | Description |
|-----|-------------|
| `site_name` | Display name of the site |
| `site_description` | Short description used in meta tags |
| `social_github` | GitHub profile or repo URL |
| `social_twitter` | Twitter/X profile URL |
| `social_linkedin` | LinkedIn profile URL |

## API

Full API reference: [docs/api.md](docs/api.md)

## Theme

Folio ships with [foliocms-theme-default](https://github.com/srmdn/foliocms-theme-default), an Astro SSR theme. The installer sets it up automatically.

To build a custom theme, see [docs/theme-contract.md](docs/theme-contract.md).

## Architecture

- **Backend**: Go, Chi router, SQLite (`modernc.org/sqlite`, pure Go, no CGo)
- **Auth**: JWT (cookie + Bearer header) + stateless CSRF (HMAC-SHA256)
- **Default theme**: Astro SSR (separate repo)
- **Content**: Markdown files with YAML frontmatter in `content/blog/<slug>/index.md`

## License

MIT
