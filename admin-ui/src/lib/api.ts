// Typed API client. Handles CSRF token fetch and injection automatically.

export interface Post {
  id: number
  slug: string
  title: string
  description: string
  tags: string[]
  draft: boolean
  publish_date: string
  created_at: string
  updated_at: string
}

export interface PostDetail extends Post {
  body: string
  hero_image: string // base64 data URI or empty
}

export interface Subscriber {
  id: number
  email: string
  token: string
  subscribed_at: string
}

export interface SiteSettings {
  site_name: string
  site_description: string
  social_github: string
  social_twitter: string
  social_linkedin: string
}

export interface MediaFile {
  key: string
  filename: string
  content_type: string
  size: number
  url: string
  created_at: string
}

export interface DemoInfo {
  demo: boolean
  email?: string
  password?: string
}

export interface RebuildStatus {
  status: 'idle' | 'running' | 'success' | 'failed'
  output: string
  started_at: string
  finished_at: string
  error: string
}

export interface PostSavePayload {
  slug: string
  title: string
  description: string
  tags: string[]
  publish_date: string
  draft: boolean
  body: string
  hero_image: string // base64 data URI or empty (empty = preserve on update)
}

let csrfToken: string | null = null

async function fetchCSRF(): Promise<string> {
  const res = await fetch('/api/csrf-token', { credentials: 'include' })
  if (!res.ok) throw new Error('unauthenticated')
  const data = await res.json()
  return data.csrf_token as string
}

async function getCSRF(): Promise<string> {
  if (!csrfToken) {
    csrfToken = await fetchCSRF()
  }
  return csrfToken
}

export function clearCSRF() {
  csrfToken = null
}

async function request<T>(
  method: string,
  path: string,
  body?: unknown,
  withCSRF = false,
): Promise<T> {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
  }

  if (withCSRF) {
    headers['X-CSRF-Token'] = await getCSRF()
  }

  const res = await fetch(path, {
    method,
    headers,
    credentials: 'include',
    body: body !== undefined ? JSON.stringify(body) : undefined,
  })

  if (res.status === 401) {
    csrfToken = null
    throw new Error('unauthenticated')
  }

  if (!res.ok) {
    const text = await res.text()
    throw new Error(text || `HTTP ${res.status}`)
  }

  if (res.status === 204 || res.headers.get('Content-Length') === '0') {
    return undefined as T
  }

  return res.json() as Promise<T>
}

export const api = {
  login: (email: string, password: string) =>
    request<void>('POST', '/api/login', { email, password }),

  logout: () => request<void>('POST', '/api/logout', undefined, true),

  checkSession: () => fetchCSRF(),

  getPosts: () => request<Post[] | null>('GET', '/api/admin/posts'),

  getPost: (slug: string) =>
    request<PostDetail>('GET', `/api/admin/posts/${slug}`),

  createPost: (slug: string, payload: PostSavePayload) =>
    request<void>('POST', `/api/admin/posts/${slug}`, payload, true),

  updatePost: (slug: string, payload: PostSavePayload) =>
    request<void>('PUT', `/api/admin/posts/${slug}`, payload, true),

  deletePost: (slug: string) =>
    request<void>('DELETE', `/api/admin/posts/${slug}`, undefined, true),

  getSubscribers: () =>
    request<Subscriber[]>('GET', '/api/admin/subscribers'),

  deleteSubscriber: (id: number) =>
    request<void>('DELETE', `/api/admin/subscribers/${id}`, undefined, true),

  triggerRebuild: () =>
    request<void>('POST', '/api/admin/rebuild', undefined, true),

  getRebuildStatus: () =>
    request<RebuildStatus>('GET', '/api/admin/rebuild/status'),

  getSettings: () =>
    request<SiteSettings>('GET', '/api/admin/settings'),

  updateSettings: (settings: SiteSettings) =>
    request<void>('PUT', '/api/admin/settings', settings, true),

  listMedia: () => request<MediaFile[]>('GET', '/api/admin/media'),

  uploadMedia: async (file: File): Promise<MediaFile> => {
    const csrf = await getCSRF()
    const form = new FormData()
    form.append('file', file)
    const res = await fetch('/api/admin/media', {
      method: 'POST',
      headers: { 'X-CSRF-Token': csrf },
      credentials: 'include',
      body: form,
    })
    if (res.status === 401) {
      csrfToken = null
      throw new Error('unauthenticated')
    }
    if (!res.ok) {
      const text = await res.text()
      throw new Error(text || `HTTP ${res.status}`)
    }
    return res.json() as Promise<MediaFile>
  },

  deleteMedia: (key: string) =>
    request<void>('DELETE', `/api/admin/media/${encodeURIComponent(key)}`, undefined, true),

  getDemo: () => request<DemoInfo>('GET', '/api/demo'),

  resetDemo: () => request<void>('POST', '/api/admin/demo/reset', undefined, true),

  changePassword: (currentPassword: string, newPassword: string) =>
    request<void>('PUT', '/api/admin/account/password', {
      current_password: currentPassword,
      new_password: newPassword,
    }, true),

  duplicatePost: (slug: string) =>
    request<{ slug: string }>('POST', `/api/admin/posts/${slug}/duplicate`, undefined, true),

  sendNewsletter: (subject: string, body: string) =>
    request<void>('POST', '/api/admin/newsletter/send', { subject, body }, true),
}
