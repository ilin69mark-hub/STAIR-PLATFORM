import { describe, expect, it } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { GeometryViewer } from './GeometryViewer'

const empty = (): any => ({ Vertices: [], Triangles: [] })

const WALLS = ['В', 'Н', 'П', 'Л'] as const

describe('GeometryViewer walls widget', () => {
  it('renders 4 wall segment buttons and exit checkbox', () => {
    render(<GeometryViewer mesh={empty()} roomWidth={3000} roomLength={4200} />)
    for (const k of WALLS) expect(screen.getByRole('button', { name: k })).toBeInTheDocument()
    expect(screen.getByRole('checkbox', { name: 'Выход на 2-й этаж' })).toBeChecked()
  })

  it('БЕЗ габаритов помещения тумблеры залочены и выводится подсказка', () => {
    render(<GeometryViewer mesh={empty()} />)
    for (const k of WALLS) {
      expect(screen.getByRole('button', { name: k })).toBeDisabled()
    }
    expect(screen.getByText(/Введите габариты помещения/i)).toBeInTheDocument()
    // Плита выхода не зависит от помещения — чекбокс остаётся активным.
    expect(screen.getByRole('checkbox', { name: 'Выход на 2-й этаж' })).not.toBeDisabled()
  })

  it('нулевые или отрицательные габариты тоже залочены', () => {
    const { unmount } = render(<GeometryViewer mesh={empty()} roomWidth={0} roomLength={4200} />)
    expect(screen.getByRole('button', { name: 'В' })).toBeDisabled()
    expect(screen.getByText(/Введите габариты помещения/i)).toBeInTheDocument()
    unmount()
    render(<GeometryViewer mesh={empty()} roomWidth={3000} roomLength={-5} />)
    expect(screen.getByRole('button', { name: 'В' })).toBeDisabled()
  })

  it('залоченные кнопки объясняют причину в title', () => {
    render(<GeometryViewer mesh={empty()} />)
    expect(screen.getByRole('button', { name: 'В' })).toHaveAttribute(
      'title',
      'Введите габариты помещения, чтобы показать стены',
    )
  })

  it('с габаритами помещения тумблеры активны, подсказка скрыта', () => {
    render(<GeometryViewer mesh={empty()} roomWidth={3000} roomLength={4200} />)
    for (const k of WALLS) {
      expect(screen.getByRole('button', { name: k })).not.toBeDisabled()
    }
    expect(screen.queryByText(/Введите габариты помещения/i)).not.toBeInTheDocument()
  })

  it('walls start off, toggling lights only the clicked side', () => {
    render(<GeometryViewer mesh={empty()} roomWidth={3000} roomLength={4200} />)
    const top = screen.getByRole('button', { name: 'В' })
    const bottom = screen.getByRole('button', { name: 'Н' })
    expect(top).toHaveAttribute('aria-pressed', 'false')
    expect(top.className).not.toContain('--on')
    fireEvent.click(top)
    expect(top).toHaveAttribute('aria-pressed', 'true')
    expect(top.className).toContain('viewer__walls__seg--on')
    expect(bottom).toHaveAttribute('aria-pressed', 'false')
    expect(bottom.className).not.toContain('--on')
    fireEvent.click(top)
    expect(top).toHaveAttribute('aria-pressed', 'false')
  })

  it('exit checkbox toggles', () => {
    render(<GeometryViewer mesh={empty()} />)
    const exit = screen.getByRole('checkbox', { name: 'Выход на 2-й этаж' })
    fireEvent.click(exit)
    expect(exit).not.toBeChecked()
    fireEvent.click(exit)
    expect(exit).toBeChecked()
  })
})