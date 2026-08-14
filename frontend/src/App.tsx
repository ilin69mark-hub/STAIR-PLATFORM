import { useCallback, useEffect, useState } from 'react'
import './App.css'
import { projectsApi } from './api/projects'
import type { Project } from './api/types'
import { ProjectList } from './components/ProjectList'
import { ProjectDetail } from './components/ProjectDetail'
import { ApiError } from './api/types'

function App() {
  const [projects, setProjects] = useState<Project[]>([])
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)

  const refresh = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      setProjects(await projectsApi.list())
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Не удалось загрузить проекты')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void refresh()
  }, [refresh])

  const handleCreated = async (p: Project) => {
    await refresh()
    setSelectedId(p.id)
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
        <h1 className="page__title">STAIR PLATFORM</h1>
        <p className="page__subtitle">MVP-09 · Критический workflow</p>
      </header>
      {error && <div className="alert alert--error">{error}</div>}
      <ProjectList
        projects={projects}
        loading={loading}
        onSelect={setSelectedId}
        onCreated={handleCreated}
      />
    </div>
  )
}

export default App
