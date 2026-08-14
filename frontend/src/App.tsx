import { useCallback, useEffect, useState } from 'react'
import './App.css'
import { projectsApi } from './api/projects'
import type { Project } from './api/types'
import { ProjectList } from './components/ProjectList'
import { ProjectDetail } from './components/ProjectDetail'
import { AuthPage } from './components/AuthPage'
import { useAuth } from './auth/context'
import { ApiError } from './api/types'

function App() {
  const { user, loading, logout } = useAuth()
  const [projects, setProjects] = useState<Project[]>([])
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [loadingList, setLoadingList] = useState(false)

  const refresh = useCallback(async () => {
    setLoadingList(true)
    setError(null)
    try {
      setProjects(await projectsApi.list())
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Не удалось загрузить проекты')
    } finally {
      setLoadingList(false)
    }
  }, [])

  useEffect(() => {
    if (user) {
      void refresh()
    }
  }, [user, refresh])

  if (loading) {
    return (
      <div className="page">
        <p className="muted">Загрузка…</p>
      </div>
    )
  }

  if (!user) {
    return <AuthPage />
  }

  const handleLogout = async () => {
    setSelectedId(null)
    await logout()
  }

  if (selectedId) {
    return (
      <ProjectDetail
        projectId={selectedId}
        onBack={() => setSelectedId(null)}
        onChanged={refresh}
      />
    )
  }

  return (
    <div className="page">
      <header className="page__header">
        <div>
          <h1 className="page__title">STAIR PLATFORM</h1>
          <p className="page__subtitle">MVP-11 · Критический workflow</p>
        </div>
        <div className="page__actions">
          <span className="muted">{user.email}</span>
          <button className="btn btn--ghost" onClick={handleLogout}>
            Выйти
          </button>
        </div>
      </header>
      {error && <div className="alert alert--error">{error}</div>}
      <ProjectList
        projects={projects}
        loading={loadingList}
        currentUserId={user.id}
        onSelect={setSelectedId}
        onCreated={(p) => {
          void refresh()
          setSelectedId(p.id)
        }}
      />
    </div>
  )
}

export default App
