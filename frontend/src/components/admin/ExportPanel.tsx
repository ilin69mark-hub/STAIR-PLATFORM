import React from 'react'

interface Props {
  onExport: (scope: string, format: 'json' | 'csv') => void
}

export const ExportPanel = React.memo(function ExportPanel({ onExport }: Props) {
  return (
    <section className="panel">
      <h2 className="panel__title">Экспорт данных</h2>
      <p className="muted">Выгрузка данных tenant в JSON или CSV (только для администратора).</p>
      <div className="row--actions">
        {(['users', 'projects', 'audit'] as const).map((scope) => (
          <span key={scope}>
            <button className="btn" onClick={() => onExport(scope, 'json')}>
              {scope} · JSON
            </button>{' '}
            <button className="btn" onClick={() => onExport(scope, 'csv')}>
              {scope} · CSV
            </button>
          </span>
        ))}
      </div>
    </section>
  )
})
