import { useEffect, useState } from 'react'
import type { Calculation, Project } from '../api/types'
import { ApiError } from '../api/types'
import { projectsApi } from '../api/projects'
import { defaultConfig, toRequest, type ConfigForm } from '../lib/config'
import { ResultPanel } from './ResultPanel'

interface Props {
  projectId: string
  onBack: () => void
  onChanged: () => void
}

export function ProjectDetail({ projectId, onBack }: Props) {
  const [project, setProject] = useState<Project | null>(null)
  const [config, setConfig] = useState<ConfigForm>(defaultConfig)
  const [calculation, setCalculation] = useState<Calculation | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    projectsApi
      .get(projectId)
      .then(setProject)
      .catch((e) => setError(e instanceof ApiError ? e.message : 'Не удалось загрузить проект'))
  }, [projectId])

  const setField = (key: keyof ConfigForm, value: string) =>
    setConfig((c) => ({ ...c, [key]: value }))

  const handleCalculate = async () => {
    setBusy(true)
    setError(null)
    try {
      setCalculation(await projectsApi.calculate(projectId, toRequest(config)))
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Не удалось выполнить расчёт')
    } finally {
      setBusy(false)
    }
  }

  const handleExport = () => {
    window.location.href = projectsApi.exportUrl(projectId)
  }

  return (
    <div className="page">
      <header className="page__header">
        <button className="btn btn--ghost" onClick={onBack}>
          ← Проекты
        </button>
        <h1 className="page__title">{project?.name ?? 'Проект'}</h1>
        <p className="page__subtitle">
          {project ? `${project.status} · создан ${new Date(project.created_at).toLocaleString('ru-RU')}` : 'Загрузка…'}
        </p>
      </header>

      {error && <div className="alert alert--error">{error}</div>}

      <div className="projects">
        <section className="panel">
          <h2 className="panel__title">Параметры лестницы</h2>
          <ConfigForm fields={config} onChange={setField} />
          <div className="row row--actions">
            <button className="btn btn--primary" onClick={handleCalculate} disabled={busy}>
              {busy ? 'Расчёт…' : 'Рассчитать'}
            </button>
            <button className="btn" onClick={handleExport} disabled={!calculation}>
              Экспорт JSON
            </button>
          </div>
          {calculation && (
            <p className="muted">
              Расчёт {calculation.valid ? 'успешен' : 'с ошибками'} ·{' '}
              {new Date(calculation.created_at).toLocaleString('ru-RU')}
            </p>
          )}
        </section>
      </div>

      {calculation && <ResultPanel snapshot={calculation.result} />}
    </div>
  )
}

interface ConfigFormProps {
  fields: ConfigForm
  onChange: (key: keyof ConfigForm, value: string) => void
}

const configFields: Array<{ key: keyof ConfigForm; label: string; hint?: string }> = [
  { key: 'widthMM', label: 'Ширина марша, мм' },
  { key: 'heightMM', label: 'Высота подъёма H, мм' },
  { key: 'stepHeightMM', label: 'Целевая высота ступени, мм' },
  { key: 'stringerThicknessMM', label: 'Толщина косоура, мм' },
  { key: 'stepThicknessMM', label: 'Толщина ступени, мм' },
  { key: 'clearanceMM', label: 'Зазор, мм' },
  { key: 'railingHeightMM', label: 'Высота ограждения, мм' },
  { key: 'comfortStepMM', label: 'Шаг комфорта S (600–640), мм', hint: 'необязательно' },
]

function ConfigForm({ fields, onChange }: ConfigFormProps) {
  return (
    <div className="config-grid">
      {configFields.map((f) => (
        <div className="field" key={f.key}>
          <label className="field__label" htmlFor={`cfg-${f.key}`}>
            {f.label}
            {f.hint && <span className="field__hint"> {f.hint}</span>}
          </label>
          <input
            id={`cfg-${f.key}`}
            className="field__input"
            type="number"
            inputMode="decimal"
            value={fields[f.key]}
            onChange={(e) => onChange(f.key, e.target.value)}
          />
        </div>
      ))}
    </div>
  )
}