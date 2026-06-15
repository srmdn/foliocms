import { useEffect, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { api } from '../lib/api'
import type { Post } from '../lib/api'

type PostStatus = 'published' | 'draft' | 'scheduled'
type FilterTab = 'all' | PostStatus

function getStatus(post: Post): PostStatus {
  if (post.draft) return 'draft'
  if (new Date(post.publish_date) > new Date()) return 'scheduled'
  return 'published'
}

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleDateString('en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
  })
}

export function Posts() {
  const [posts, setPosts] = useState<Post[] | null>(null)
  const [error, setError] = useState('')
  const [confirmSlug, setConfirmSlug] = useState<string | null>(null)
  const [deleting, setDeleting] = useState(false)
  const [duplicating, setDuplicating] = useState<string | null>(null)
  const [filter, setFilter] = useState<FilterTab>('all')
  const navigate = useNavigate()

  useEffect(() => {
    loadPosts()
  }, [])

  async function loadPosts() {
    try {
      const data = await api.getPosts()
      setPosts(data ?? [])
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load posts')
    }
  }

  async function handleDuplicate(slug: string) {
    setDuplicating(slug)
    try {
      const { slug: newSlug } = await api.duplicatePost(slug)
      navigate(`/admin/posts/${newSlug}/edit`)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to duplicate post')
    } finally {
      setDuplicating(null)
    }
  }

  async function handleDelete(slug: string) {
    setDeleting(true)
    try {
      await api.deletePost(slug)
      setPosts((prev) => prev?.filter((p) => p.slug !== slug) ?? [])
      setConfirmSlug(null)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to delete post')
    } finally {
      setDeleting(false)
    }
  }

  const allPosts = posts ?? []
  const counts = {
    all: allPosts.length,
    published: allPosts.filter((p) => getStatus(p) === 'published').length,
    draft: allPosts.filter((p) => getStatus(p) === 'draft').length,
    scheduled: allPosts.filter((p) => getStatus(p) === 'scheduled').length,
  }
  const visible = filter === 'all' ? allPosts : allPosts.filter((p) => getStatus(p) === filter)

  return (
    <>
      <div className="page-header">
        <h1>Posts</h1>
        <Link to="/admin/posts/new" className="btn-sm btn-sm-primary">
          + New post
        </Link>
      </div>

      <div className="page-content">
        {error && <p className="state-error">{error}</p>}

        {posts === null && !error && (
          <div className="state-center">
            <div className="spinner" />
          </div>
        )}

        {posts !== null && (
          <>
            {/* Filter tabs */}
            <div className="filter-tabs">
              {(['all', 'published', 'draft', 'scheduled'] as FilterTab[]).map((tab) => (
                <button
                  key={tab}
                  className={`filter-tab${filter === tab ? ' filter-tab-active' : ''}`}
                  onClick={() => setFilter(tab)}
                >
                  {tab.charAt(0).toUpperCase() + tab.slice(1)}
                  <span className="filter-tab-count">{counts[tab]}</span>
                </button>
              ))}
            </div>

            {visible.length === 0 && (
              <div className="state-center state-muted">
                {filter === 'all'
                  ? <span>No posts yet. <Link to="/admin/posts/new">Create your first post.</Link></span>
                  : <span>No {filter} posts.</span>
                }
              </div>
            )}

            {visible.length > 0 && (
              <div className="post-list">
                {visible.map((post) => {
                  const status = getStatus(post)
                  const tags = post.tags ?? []
                  return (
                    <div
                      key={post.slug}
                      className="post-item"
                      onClick={() => navigate(`/admin/posts/${post.slug}/edit`)}
                    >
                      <div className="post-info">
                        <div className="post-title">{post.title}</div>
                        {post.description && (
                          <div className="post-desc">{post.description}</div>
                        )}
                        <div className="post-sub">
                          <span className={`badge badge-${status}`}>{status}</span>
                          <span className="post-date">{formatDate(post.publish_date)}</span>
                          {tags.length > 0 && (
                            <span className="post-tags">
                              {tags.map((tag) => (
                                <span key={tag} className="post-tag">{tag}</span>
                              ))}
                            </span>
                          )}
                        </div>
                      </div>

                      <div className="post-actions" onClick={(e) => e.stopPropagation()}>
                        {confirmSlug === post.slug ? (
                          <div className="confirm-inline">
                            <span>Delete?</span>
                            <button
                              className="btn-sm btn-sm-danger"
                              onClick={() => handleDelete(post.slug)}
                              disabled={deleting}
                            >
                              {deleting ? '...' : 'Yes'}
                            </button>
                            <button
                              className="btn-sm btn-sm-ghost"
                              onClick={() => setConfirmSlug(null)}
                              disabled={deleting}
                            >
                              Cancel
                            </button>
                          </div>
                        ) : (
                          <>
                            <Link
                              to={`/admin/posts/${post.slug}/edit`}
                              className="btn-sm btn-sm-ghost"
                              onClick={(e) => e.stopPropagation()}
                            >
                              Edit
                            </Link>
                            <button
                              className="btn-sm btn-sm-ghost"
                              onClick={() => handleDuplicate(post.slug)}
                              disabled={duplicating === post.slug}
                            >
                              {duplicating === post.slug ? '...' : 'Duplicate'}
                            </button>
                            <button
                              className="btn-sm btn-sm-danger"
                              onClick={() => setConfirmSlug(post.slug)}
                            >
                              Delete
                            </button>
                          </>
                        )}
                      </div>
                    </div>
                  )
                })}
              </div>
            )}
          </>
        )}
      </div>
    </>
  )
}
