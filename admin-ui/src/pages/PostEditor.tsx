import { useState, useEffect, useRef } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { api } from '../lib/api'
import { MarkdownEditor } from '../components/Editor'

function generateSlug(title: string): string {
  return title
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9\s-]/g, '')
    .replace(/\s+/g, '-')
    .replace(/-+/g, '-')
    .replace(/^-|-$/g, '')
    .slice(0, 80)
}

function todayDate(): string {
  return new Date().toISOString().slice(0, 10)
}

export function PostEditor() {
  const { slug } = useParams<{ slug: string }>()
  const isEdit = !!slug
  const navigate = useNavigate()

  const [loading, setLoading] = useState(isEdit)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const [success, setSuccess] = useState('')

  const [newsletterState, setNewsletterState] = useState<'idle' | 'confirming' | 'sending' | 'sent' | 'error'>('idle')
  const [newsletterError, setNewsletterError] = useState('')

  const [title, setTitle] = useState('')
  const [postSlug, setPostSlug] = useState('')
  const [slugManual, setSlugManual] = useState(false)
  const [description, setDescription] = useState('')
  const [tags, setTags] = useState('')
  const [publishDate, setPublishDate] = useState(todayDate())
  const [draft, setDraft] = useState(true)
  const [body, setBody] = useState('')
  const [heroImage, setHeroImage] = useState('')
  const [heroChanged, setHeroChanged] = useState(false)
  const heroInputRef = useRef<HTMLInputElement>(null)
  const handleSaveRef = useRef<() => void>(() => {})

  useEffect(() => {
    if (isEdit && slug) loadPost(slug)
  }, [isEdit, slug])

  // Cmd/Ctrl+S shortcut
  useEffect(() => {
    function onKeyDown(e: KeyboardEvent) {
      if ((e.metaKey || e.ctrlKey) && e.key === 's') {
        e.preventDefault()
        handleSaveRef.current()
      }
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [])

  async function loadPost(s: string) {
    try {
      const post = await api.getPost(s)
      setTitle(post.title)
      setPostSlug(post.slug)
      setSlugManual(true)
      setDescription(post.description ?? '')
      setTags((post.tags ?? []).join(', '))
      setPublishDate(post.publish_date.slice(0, 10))
      setDraft(post.draft)
      setBody(post.body ?? '')
      setHeroImage(post.hero_image ?? '')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load post')
    } finally {
      setLoading(false)
    }
  }

  function handleTitleChange(v: string) {
    setTitle(v)
    if (!slugManual) setPostSlug(generateSlug(v))
  }

  function handleSlugChange(v: string) {
    setPostSlug(v)
    setSlugManual(true)
  }

  function handleHeroChange(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0]
    if (!file) return
    const reader = new FileReader()
    reader.onload = () => {
      setHeroImage(reader.result as string)
      setHeroChanged(true)
    }
    reader.readAsDataURL(file)
  }

  function clearHero() {
    setHeroImage('')
    setHeroChanged(true)
    if (heroInputRef.current) heroInputRef.current.value = ''
  }

  async function handleSave() {
    if (!postSlug) { setError('Slug is required'); return }
    if (!title) { setError('Title is required'); return }

    setSaving(true)
    setError('')
    setSuccess('')

    const tagsArray = tags.split(',').map((t) => t.trim()).filter(Boolean)
    const payload = {
      slug: postSlug,
      title,
      description,
      tags: tagsArray,
      publish_date: publishDate,
      draft,
      body,
      hero_image: heroChanged ? heroImage : '',
    }

    try {
      if (isEdit && slug) {
        await api.updatePost(slug, payload)
        if (postSlug !== slug) {
          navigate(`/admin/posts/${postSlug}/edit`, { replace: true })
        } else {
          setSuccess('Saved')
          setTimeout(() => setSuccess(''), 3000)
        }
      } else {
        await api.createPost(postSlug, payload)
        navigate(`/admin/posts/${postSlug}/edit`, { replace: true })
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save')
    } finally {
      setSaving(false)
    }
  }

  async function handleSendNewsletter() {
    setNewsletterState('sending')
    setNewsletterError('')
    try {
      await api.sendNewsletter(title, body)
      setNewsletterState('sent')
    } catch (err) {
      setNewsletterError(err instanceof Error ? err.message : 'Failed to send')
      setNewsletterState('error')
    }
  }

  // Keep ref in sync with latest closure so the keydown handler always calls
  // the current version of handleSave (which closes over current state).
  handleSaveRef.current = handleSave

  if (loading) {
    return (
      <div className="state-center">
        <div className="spinner" />
      </div>
    )
  }

  return (
    <>
      <div className="page-header">
        <div className="page-header-left">
          <Link to="/admin/posts" className="back-link">← Posts</Link>
          <h1>{isEdit ? 'Edit post' : 'New post'}</h1>
        </div>
        <div className="page-header-right">
          {success && <span className="save-success">{success}</span>}
          {error && <span className="save-error">{error}</span>}
          <button
            className="btn-sm btn-sm-primary"
            onClick={handleSave}
            disabled={saving}
          >
            {saving ? 'Saving...' : 'Save'}
          </button>
        </div>
      </div>

      <div className="page-content editor-layout">

        {/* Main column: title + body */}
        <div className="editor-main">
          <input
            className="editor-title-input"
            type="text"
            value={title}
            onChange={(e) => handleTitleChange(e.target.value)}
            placeholder="Post title"
          />
          <MarkdownEditor value={body} onChange={setBody} />
        </div>

        {/* Side column: metadata */}
        <aside className="editor-meta">

          <div className="meta-section">
            <div className="field">
              <label htmlFor="slug">Slug</label>
              <input
                id="slug"
                type="text"
                value={postSlug}
                onChange={(e) => handleSlugChange(e.target.value)}
                placeholder="post-slug"
              />
            </div>

            <div className="field">
              <label htmlFor="description">Description</label>
              <textarea
                id="description"
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                placeholder="Short description for SEO and previews"
                rows={3}
              />
              <span className={`field-hint field-hint-right${description.length > 160 ? ' field-hint-over' : ''}`}>
                {description.length} / 160
              </span>
            </div>

            <div className="field">
              <label htmlFor="tags">Tags</label>
              <input
                id="tags"
                type="text"
                value={tags}
                onChange={(e) => setTags(e.target.value)}
                placeholder="go, cms, tutorial"
              />
              <span className="field-hint">Comma-separated</span>
            </div>

            <div className="field">
              <label htmlFor="publish-date">Publish date</label>
              <input
                id="publish-date"
                type="date"
                value={publishDate}
                onChange={(e) => setPublishDate(e.target.value)}
              />
            </div>

            <div className="field field-row">
              <label htmlFor="draft" className="label-inline">
                <input
                  id="draft"
                  type="checkbox"
                  checked={draft}
                  onChange={(e) => setDraft(e.target.checked)}
                />
                Draft
              </label>
            </div>
          </div>

          <div className="meta-section">
            <div className="field">
              <label>Hero image</label>
              {heroImage ? (
                <div className="hero-preview">
                  <img src={heroImage} alt="Hero preview" />
                  <button type="button" className="btn-sm btn-sm-ghost hero-remove" onClick={clearHero}>
                    Remove
                  </button>
                </div>
              ) : (
                <label className="hero-upload-label">
                  <input
                    ref={heroInputRef}
                    type="file"
                    accept="image/jpeg,image/png,image/webp,image/gif"
                    onChange={handleHeroChange}
                    style={{ display: 'none' }}
                  />
                  <span className="hero-upload-btn">Choose image</span>
                </label>
              )}
            </div>
          </div>

          {isEdit && !draft && (
            <div className="meta-section">
              <div className="field-group-label">Newsletter</div>
              {newsletterState === 'sent' && (
                <p className="state-success">Sent to subscribers.</p>
              )}
              {newsletterState === 'error' && (
                <p className="state-error">{newsletterError}</p>
              )}
              {newsletterState === 'confirming' && (
                <div className="confirm-inline">
                  <span>Send "{title}" to all subscribers?</span>
                  <button
                    className="btn-sm btn-sm-primary"
                    onClick={handleSendNewsletter}
                  >
                    Send
                  </button>
                  <button
                    className="btn-sm btn-sm-ghost"
                    onClick={() => setNewsletterState('idle')}
                  >
                    Cancel
                  </button>
                </div>
              )}
              {(newsletterState === 'idle' || newsletterState === 'error') && (
                <button
                  className="btn-sm btn-sm-primary"
                  onClick={() => setNewsletterState('confirming')}
                >
                  Send to subscribers
                </button>
              )}
            </div>
          )}

        </aside>
      </div>
    </>
  )
}
