import { useCallback, useEffect, useMemo, useState } from 'react'
import './App.css'

import type { Project } from '@shared/types'
import { ApiError } from '@shared/types'
import { findTile, firstTileId, type CabinetTile } from '@shared/cabinetTiles'

import { projectsApi } from './api/projects'
import { ProjectDetail } from './components/ProjectDetail'
import { ProjectList } from './components/ProjectList'
import { AuthPage } from './components/AuthPage'
import { CabinetShell } from './components/CabinetShell'
import { HomeScreen } from './components/HomeScreen'
import { TilePage } from './components/TilePage'
import { useAuth } from './auth/context'

/**
 * Кабинет: приветственный экран с плитками → страница плитки → проект.
 *
 * Раньше верхним уровнем были вкладки «Проекты / Администрирование / Аудит /
 * Магазин» — они отражали устройство системы, а не работу человека. Теперь
 * верхний уровень — блоки бизнеса (плитки), а разделы спрятаны внутрь (Editable
 * по правам: см. shared/src/cabinetTiles).
 *
 * Адрес не меняется: роутера в проекте нет (ни react-router, ни history API
 * для навигации), навигация — состояние. Терять нечего, deep-link на плитку не
 * существовало и раньше.
 */
function App() {
  const { user, loading, logout } = useAuth()
  const [projects, setProjects] = useState<Project[]>([])
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const [openTileId, setOpenTileId] = useState<string | null>(null)
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

  // Плитка открывается только если она ещё доступна: выдали доступ, забрали —
  // открытая плитка закрывается сама, а не остаётся с пустым экраном.
  const openTile: CabinetTile | null = useMemo(
    () => (openTileId ? findTile(user, openTileId) : null),
    [openTileId, user],
  )
  useEffect(() => {
    if (openTileId && !openTile) {
      setOpenTileId(firstTileId(user))
    }
  }, [openTileId, openTile, user])

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
    setOpenTileId(null)
    await logout()
  }

  const goHome = () => {
    setOpenTileId(null)
    setSelectedId(null)
  }

  if (selectedId) {
    return (
      <ProjectDetail
        projectId={selectedId}
        onBack={goHome}
        onChanged={refresh}
      />
    )
  }

  // Плитка «Проекты» — рабочее место: список и создание, а не просто кнопка.
  if (openTile?.id === 'projects') {
    return (
      <CabinetShell
        title="Проекты"
        subtitle="Расчёты лестниц, чертежи, комментарии"
        email={user.email}
        onBack={goHome}
        onLogout={handleLogout}
      >
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
      </CabinetShell>
    )
  }

  if (openTile) {
    return (
      <CabinetShell
        title={openTile.title}
        subtitle={openTile.hint}
        email={user.email}
        onBack={goHome}
        onLogout={handleLogout}
      >
        <TilePage user={user} tile={openTile} />
      </CabinetShell>
    )
  }

  return (
    <CabinetShell
      title="STAIR PLATFORM"
      subtitle="Выберите блок, за который отвечаете"
      email={user.email}
      onLogout={handleLogout}
    >
      <HomeScreen user={user} onOpenTile={(t) => setOpenTileId(t.id)} />
    </CabinetShell>
  )
}

export default App