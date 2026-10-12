import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'

import { ExportPanel } from './ExportPanel'

describe('ExportPanel', () => {
  it('предлагает выгрузку каждого раздела в оба формата', async () => {
    const user = userEvent.setup()
    const onExport = vi.fn()
    render(<ExportPanel onExport={onExport} />)

    // Три раздела × два формата — шесть кнопок. Список не должен молча
    // потерять раздел: иначе выгрузить его просто нечем.
    for (const scope of ['users', 'projects', 'audit']) {
      expect(screen.getByRole('button', { name: `${scope} · JSON` })).toBeInTheDocument()
      expect(screen.getByRole('button', { name: `${scope} · CSV` })).toBeInTheDocument()
    }

    await user.click(screen.getByRole('button', { name: 'projects · CSV' }))
    expect(onExport).toHaveBeenCalledWith('projects', 'csv')
  })

  it('передаёт выбранный формат, а не первый в списке', async () => {
    const user = userEvent.setup()
    const onExport = vi.fn()
    render(<ExportPanel onExport={onExport} />)

    // Ошибка «всегда JSON» невозможна, если клик идёт по самой кнопке CSV:
    // обработчик замыкается на формат, а не на позицию.
    await user.click(screen.getByRole('button', { name: 'users · CSV' }))
    expect(onExport).toHaveBeenLastCalledWith('users', 'csv')

    await user.click(screen.getByRole('button', { name: 'audit · JSON' }))
    expect(onExport).toHaveBeenLastCalledWith('audit', 'json')
    expect(onExport).toHaveBeenCalledTimes(2)
  })

  it('перерисовывается при смене обработчика', async () => {
    const user = userEvent.setup()
    const first = vi.fn()
    const second = vi.fn()
    const { rerender } = render(<ExportPanel onExport={first} />)
    rerender(<ExportPanel onExport={second} />)

    await user.click(screen.getByRole('button', { name: 'users · JSON' }))
    // React.memo сравнивает пропсы по ссылке: сменился обработчик — компонент
    // обязан перерисоваться и звать НОВЫЙ, иначе панель выгружала бы в никуда.
    expect(first).not.toHaveBeenCalled()
    expect(second).toHaveBeenCalledWith('users', 'json')
  })
})