# API Reference

Base URL: wherever Folio is running (e.g. `http://localhost:8090`)

All responses are JSON. All request bodies must be `Content-Type: application/json`.

---

## Authentication

Protected endpoints require a valid JWT. The token is issued on login and
stored in an `HttpOnly` cookie named `token`. API clients may also pass it
as a `Authorization: Bearer <token>` header.

Protected mutation endpoints (POST/PUT/DELETE) additionally require a
`X-CSRF-Token` header. The CSRF token is returned in the login response and
can be refreshed via `GET /api/csrf-token`.

---

## Public Endpoints

### `GET /health`

Returns `200 OK` with body `ok`. Use for uptime checks.

---

### `POST /api/login`

Authenticate and receive a session token.

**Request body**
```json
{
  "email": "admin@example.com",
  "password": "yourpassword"
}
```

**Response `200`**
```json
{
  "csrf_token": "abc123..."
}
```

Sets an `HttpOnly` cookie named `token` (24-hour TTL).
Store the `csrf_token` and include it as `X-CSRF-Token` on all subsequent
mutation requests.

**Errors**: `400` invalid body, `401` invalid credentials

---

### `POST /api/logout`

Clear the session cookie.

**Response `204`** No content.

---

### `GET /api/posts`

List all published posts. Does not include post body.

**Query parameters**

| Name | Required | Description |
|------|----------|-------------|
| `tag` | No | Exact-match tag filter. `GET /api/posts?tag=go` returns only posts tagged `go`. |

**Response `200`**
```json
[
  {
    "id": 1,
    "slug": "my-first-post",
    "title": "My First Post",
    "description": "A short description.",
    "tags": ["go", "cms"],
    "draft": false,
    "publish_date": "2026-03-27T00:00:00Z",
    "created_at": "2026-03-27T10:00:00Z",
    "updated_at": "2026-03-27T10:00:00Z"
  }
]
```

Returns `[]` (empty array) if no posts are published.

---

### `GET /api/posts/{slug}`

Get a single published post including its Markdown body.


**Response `200`**
```json
{
  "id": 1,
  "slug": "my-first-post",
  "title": "My First Post",
  "description": "A short description.",
  "tags": ["go", "cms"],
  "draft": false,
  "publish_date": "2026-03-27T00:00:00Z",
  "created_at": "2026-03-27T10:00:00Z",
  "updated_at": "2026-03-27T10:00:00Z",
  "body": "Post body in **Markdown**."
}
```

**Errors**: `404` post not found or is a draft

---

### `POST /api/subscribe`

Subscribe an email address to the newsletter.

**Request body**
```json
{ "email": "reader@example.com" }
```

**Response `201`** No content.

**Errors**: `400` missing email, `409` already subscribed

---

### `GET /api/unsubscribe`

Unsubscribe using the token from a newsletter email.

**Query params**: `?token=<unsubscribe-token>`

**Response `204`** No content.

**Errors**: `400` missing token, `404` token not found

---

## Authenticated Endpoints

Requires: valid `token` cookie or `Authorization: Bearer` header.

---

### `GET /api/csrf-token`

Get the CSRF token for the current session. Use this after a page refresh
when the token stored in memory is lost.

**Response `200`**
```json
{
  "csrf_token": "abc123..."
}
```

---

## Authenticated + CSRF Endpoints

Requires: valid `token` cookie/header **and** `X-CSRF-Token` header.

---

### `GET /api/admin/posts`

List all posts including drafts.

**Response `200`**: same shape as `GET /api/posts` but includes drafts.

---

### `GET /api/admin/posts/{slug}`

Get a single post regardless of draft status, including body.

**Response `200`**: same shape as `GET /api/posts/{slug}`.

**Errors**: `404` post not found

---

### `POST /api/admin/posts/{slug}`

Create a new post. The slug is set in the URL path.

**Slug rules**: lowercase letters, numbers, and hyphens only. Example: `my-first-post`

**Request body**
```json
{
  "title": "My First Post",
  "description": "A short description.",
  "tags": ["go", "cms"],
  "draft": true,
  "publish_date": "2026-03-27",
  "body": "Post body in **Markdown**."
}
```

| Field | Required | Notes |
|-------|----------|-------|
| `title` | Yes | |
| `description` | No | |
| `tags` | No | Array of strings |
| `draft` | No | Defaults to `false` |
| `publish_date` | No | Format `YYYY-MM-DD`. Defaults to today. |
| `body` | No | Markdown string |

**Response `201`** No content.

**Errors**: `400` invalid slug or missing title, `409` post already exists

---

### `PUT /api/admin/posts/{slug}`

Update an existing post. Replaces all fields.

**Request body**: same shape as `POST /api/admin/posts/{slug}`

**Response `204`** No content.

**Errors**: `400` missing title, `404` post not found

---

### `DELETE /api/admin/posts/{slug}`

Delete a post and its content files from disk.

**Response `204`** No content.

**Errors**: `404` post not found

---

### `GET /api/admin/subscribers`

List all newsletter subscribers.

**Response `200`**
```json
[
  {
    "id": 1,
    "email": "reader@example.com",
    "token": "uuid-unsubscribe-token",
    "subscribed_at": "2026-03-27T10:00:00Z"
  }
]
```

Returns `[]` if no subscribers.

---

### `DELETE /api/admin/subscribers/{id}`

Remove a subscriber by ID.

**Response `204`** No content.

**Errors**: `404` subscriber not found

---

### `POST /api/admin/newsletter/send`

Send an email to all subscribers. Requires SMTP to be configured.

**Request body**
```json
{
  "subject": "New post: My First Post",
  "body": "Email body in plain text or HTML."
}
```

**Response `200`**
```json
{ "sent": 42 }
```

**Errors**: `400` missing subject or body, `503` SMTP not configured

---

### `POST /api/webhook/rebuild`

Trigger a theme rebuild via a shared secret. Disabled (404) if
`WEBHOOK_SECRET` is not set in `.env`.

Pass the secret via `X-Webhook-Secret` header or `Authorization: Bearer <secret>`.

**Response `202`** Build started.

**Response `401`** Invalid secret.

**Response `409`** Rebuild already in progress.

---

### `POST /api/admin/rebuild`

Trigger an async theme rebuild. Returns immediately; poll status to track
progress.

**Response `202`** Build started.

**Response `409`** A rebuild is already in progress.

---

### `GET /api/admin/rebuild/status`

Get the current rebuild status.

**Response `200`**
```json
{
  "status": "success",
  "output": "...",
  "started_at": "2026-03-27T10:00:00Z",
  "finished_at": "2026-03-27T10:01:05Z",
  "error": ""
}
```

| `status` value | Meaning |
|----------------|---------|
| `idle` | No build has run yet |
| `running` | Build in progress |
| `success` | Last build succeeded |
| `failed` | Last build failed; see `error` field |

---

### `GET /api/settings`

Get all site settings. Public endpoint — intended for themes to read site metadata.

**Response `200`**
```json
{
  "site_name": "My Blog",
  "site_description": "A personal blog about Go and systems.",
  "social_github": "https://github.com/example",
  "social_twitter": "https://twitter.com/example",
  "social_linkedin": ""
}
```

Returns a flat key/value object. Keys with no value set return an empty string.

---

### `GET /api/admin/settings`

Get all site settings (protected). Same response shape as `GET /api/settings`.

---

### `PUT /api/admin/settings`

Update one or more site settings (protected + CSRF).

**Request body**
```json
{
  "site_name": "My Blog",
  "site_description": "A personal blog about Go and systems.",
  "social_github": "https://github.com/example",
  "social_twitter": "",
  "social_linkedin": ""
}
```

Only known keys are accepted. Unknown keys return `400`.

| Key | Description |
|-----|-------------|
| `site_name` | Display name of the site |
| `site_description` | Short description used in meta tags |
| `social_github` | GitHub profile or repo URL |
| `social_twitter` | Twitter/X profile URL |
| `social_linkedin` | LinkedIn profile URL |

**Response `204`** No content.

**Errors**: `400` invalid JSON or unknown key

---

### `POST /api/admin/media`

Upload an image file (protected + CSRF). Accepts `multipart/form-data`.

**Form field**: `file` — the image to upload.

**Constraints**:
- Max file size: 10 MB
- Allowed types: `image/jpeg`, `image/png`, `image/gif`, `image/webp`, `image/svg+xml`
- Content type is detected from file bytes, not the `Content-Type` header

**Response `201`**
```json
{
  "key": "550e8400-e29b-41d4-a716-446655440000-photo.jpg",
  "filename": "photo.jpg",
  "content_type": "image/jpeg",
  "size": 204800,
  "url": "https://example.com/media/550e8400-e29b-41d4-a716-446655440000-photo.jpg",
  "created_at": "2026-04-01T10:00:00Z"
}
```

**Errors**: `400` missing file or invalid form, `415` unsupported file type, `413` file too large

---

### `GET /api/admin/media`

List all uploaded files (protected + CSRF). Returns newest first.

**Response `200`**
```json
[
  {
    "key": "550e8400-e29b-41d4-a716-446655440000-photo.jpg",
    "filename": "photo.jpg",
    "content_type": "image/jpeg",
    "size": 204800,
    "url": "https://example.com/media/550e8400-e29b-41d4-a716-446655440000-photo.jpg",
    "created_at": "2026-04-01T10:00:00Z"
  }
]
```

Returns `[]` if no files have been uploaded.

---

### `DELETE /api/admin/media/{key}`

Delete an uploaded file by key (protected + CSRF). Removes the file from storage and the database record.

**Response `204`** No content.

**Errors**: `400` missing key, `500` storage or database error

---

## Static File Serving

### `GET /media/{key}`

Serves an uploaded file by key. Only available when `MEDIA_STORAGE=local` (default).
When using S3-compatible storage, files are served directly from the bucket via the `url` field
returned by the upload and list endpoints.
