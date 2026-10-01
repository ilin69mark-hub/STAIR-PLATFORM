import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { ProjectDetail } from './ProjectDetail'
import { projectsApi } from '../api/projects'
import { ApiError } from '@shared/types'
import { makeCalculation, makeOptimize, makeProject, makeSnapshot } from '../test/fixtures'

// Захват пропсов GeometryViewer (сам вьювер тестируется в shared): проверяем
// сквозную связку «поле Высота подъёма H, мм в калькуляторе → ResultPanel →
// высота стен 3D».
const viewerProps = vi.hoisted(() => ({ current: {} as Record<string, unknown> }))
vi.mock('@shared/viewer/GeometryViewer', () => ({
  GeometryViewer: (p: Record<string, unknown>) => {
    viewerProps.current = p
    return null
  },
}))

const mesh3d = {
  Vertices: [
    { X: 0, Y: 0, Z: 0 },
    { X: 900, Y: 0, Z: 0 },
    { X: 0, Y: 0, Z: 2700 },
  ],
  Triangles: [[0, 1, 2]] as Array<[number, number, number]>,
}

afterEach(() => {
  vi.restoreAllMocks()
})

function renderDetail() {
  const onChanged = vi.fn()
  const onBack = vi.fn()
  const view = render(
    <ProjectDetail projectId="p1" onBack={onBack} onChanged={onChanged} />,
  )
  return { onChanged, onBack, view }
}

describe('ProjectDetail', () => {
  it('загружает и показывает проект', async () => {
    vi.spyOn(projectsApi, 'get').mockResolvedValue(makeProject())
    renderDetail()
    expect(await screen.findByRole('heading', { name: 'Лестница на второй этаж' })).toBeInTheDocument()
    expect(screen.getByText(/создан/)).toBeInTheDocument()
  })

  it('показывает ошибку при неудачной загрузке', async () => {
    vi.spyOn(projectsApi, 'get').mockRejectedValue(new ApiError(404, 'not_found', 'Нет проекта'))
    renderDetail()
    expect(await screen.findByText('Нет проекта')).toBeInTheDocument()
  })

  it('блокирует расчёт при жёстких ошибках валидации', async () => {
    vi.spyOn(projectsApi, 'get').mockResolvedValue(makeProject())
    renderDetail()
    const width = await screen.findByLabelText(/Ширина марша/)
    fireEvent.change(width, { target: { value: '' } })

    expect(screen.getByText('Укажите значение')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Рассчитать' })).toBeDisabled()
    expect(screen.getByText(/Исправьте нечисловые или пустые поля/)).toBeInTheDocument()
  })

  it('выполняет расчёт, показывает индикатор сохранения и зовёт onChanged', async () => {
    vi.spyOn(projectsApi, 'get').mockResolvedValue(makeProject())
    const calculate = vi.spyOn(projectsApi, 'calculate').mockResolvedValue(makeCalculation())
    const { onChanged } = renderDetail()

    await screen.findByRole('heading', { name: 'Лестница на второй этаж' })
    expect(screen.getByRole('button', { name: 'Коммерческое предложение (PDF)' })).toBeDisabled()

    fireEvent.click(screen.getByRole('button', { name: 'Рассчитать' }))

    expect(await screen.findByText(/Расчёт сохранён/)).toBeInTheDocument()
    expect(onChanged).toHaveBeenCalledTimes(1)
    expect(screen.getByRole('button', { name: 'Коммерческое предложение (PDF)' })).toBeEnabled()

    const [id, body] = calculate.mock.calls[0]
    expect(id).toBe('p1')
    expect((body as Record<string, unknown>).width_mm).toBe(900)
    expect((body as Record<string, unknown>).material).toBe('STEEL-S235')
    expect((body as Record<string, unknown>).rates).toBeUndefined()
  })

  it('позволяет выбрать материал и передаёт его в расчёт', async () => {
    vi.spyOn(projectsApi, 'get').mockResolvedValue(makeProject())
    const calculate = vi.spyOn(projectsApi, 'calculate').mockResolvedValue(makeCalculation())
    renderDetail()

    await screen.findByRole('heading', { name: 'Лестница на второй этаж' })
    const select = screen.getByLabelText('Материал') as HTMLSelectElement
    expect(select.value).toBe('STEEL-S235')
    fireEvent.change(select, { target: { value: 'WOOD-OAK' } })
    fireEvent.click(screen.getByRole('button', { name: 'Рассчитать' }))
    await screen.findByText(/Расчёт сохранён/)

    const body = calculate.mock.calls[0][1] as Record<string, unknown>
    expect(body.material).toBe('WOOD-OAK')
  })

  it('включает rates в тело расчёта при заполненных ставках', async () => {
    vi.spyOn(projectsApi, 'get').mockResolvedValue(makeProject())
    const calculate = vi.spyOn(projectsApi, 'calculate').mockResolvedValue(makeCalculation())
    renderDetail()

    await screen.findByRole('heading', { name: 'Лестница на второй этаж' })
    fireEvent.change(screen.getByLabelText(/Сталь/), { target: { value: '200' } })
    fireEvent.click(screen.getByRole('button', { name: 'Рассчитать' }))
    await screen.findByText(/Расчёт сохранён/)

    const body = calculate.mock.calls[0][1] as { rates?: { material_per_kg_rub?: Record<string, number> } }
    expect(body.rates?.material_per_kg_rub?.['STEEL-S235']).toBe(200)
  })

  it('показывает ошибку при неудачном расчёте', async () => {
    vi.spyOn(projectsApi, 'get').mockResolvedValue(makeProject())
    vi.spyOn(projectsApi, 'calculate').mockRejectedValue(
      new ApiError(500, 'pipeline', 'Не удалось рассчитать'),
    )
    renderDetail()

    await screen.findByRole('heading', { name: 'Лестница на второй этаж' })
    fireEvent.click(screen.getByRole('button', { name: 'Рассчитать' }))
    expect(await screen.findByText('Не удалось рассчитать')).toBeInTheDocument()
  })

  it('оптимизирует, применяет лучшую конфигурацию и зовёт onChanged', async () => {
    vi.spyOn(projectsApi, 'get').mockResolvedValue(makeProject())
    const optimize = vi.spyOn(projectsApi, 'optimize').mockResolvedValue(makeOptimize())
    const { onChanged } = renderDetail()

    await screen.findByRole('heading', { name: 'Лестница на второй этаж' })
    fireEvent.click(screen.getByRole('button', { name: 'Оптимизировать' }))

    expect(await screen.findByText(/Оптимум найден/)).toBeInTheDocument()
    expect(onChanged).toHaveBeenCalled()
    expect(screen.getByLabelText(/Целевая высота ступени/)).toHaveValue(168.75)

    const body = optimize.mock.calls[0][1] as Record<string, unknown>
    expect(body.target).toBe('price')
    expect(body.width_mm).toBe(900)
  })

  it('показывает сообщение, когда оптимизация не находит конфигурацию', async () => {
    vi.spyOn(projectsApi, 'get').mockResolvedValue(makeProject())
    vi.spyOn(projectsApi, 'optimize').mockResolvedValue(
      makeOptimize({ valid: false, best: undefined, saved: false }),
    )
    renderDetail()

    await screen.findByRole('heading', { name: 'Лестница на второй этаж' })
    fireEvent.click(screen.getByRole('button', { name: 'Оптимизировать' }))
    expect(await screen.findByText(/не нашла допустимую конфигурацию/)).toBeInTheDocument()
  })

  it('показывает только поля, относящиеся к типу L', async () => {
    vi.spyOn(projectsApi, 'get').mockResolvedValue(makeProject())
    renderDetail()
    await screen.findByRole('heading', { name: 'Лестница на второй этаж' })

    fireEvent.change(screen.getByLabelText('Тип марша'), { target: { value: 'l_shape' } })

    expect(screen.getByLabelText(/Ширина площадки Wp/)).toBeInTheDocument()
    expect(screen.getByLabelText(/Ступеней нижнего марша/)).toBeInTheDocument()
    expect(screen.queryByLabelText(/Наружный радиус R/)).not.toBeInTheDocument()
  })

  it('спиральный марш скрыт из выбора типов (S-152)', async () => {
    vi.spyOn(projectsApi, 'get').mockResolvedValue(makeProject())
    renderDetail()
    await screen.findByRole('heading', { name: 'Лестница на второй этаж' })

    const options = Array.from(
      screen.getByLabelText('Тип марша').querySelectorAll('option'),
    ).map((o) => (o as HTMLOptionElement).value)
    expect(options).toEqual(['straight', 'l_shape', 'u_shape'])
    expect(options).not.toContain('spiral')
    // Поля площадки (спиральные) в форме тоже нет.
    expect(screen.queryByLabelText(/Наружный радиус R/)).not.toBeInTheDocument()
  })

  it('проект, сохранённый со спиралью, открывается без падения', async () => {
    vi.spyOn(projectsApi, 'get').mockResolvedValue(makeProject())
    renderDetail()
    await screen.findByRole('heading', { name: 'Лестница на второй этаж' })
    expect(screen.getByLabelText('Тип марша')).toBeInTheDocument()
    expect(screen.getByLabelText(/Ширина марша/)).toBeInTheDocument()
  })

  it('прямой марш скрывает радиус и поля площадки', async () => {
    vi.spyOn(projectsApi, 'get').mockResolvedValue(makeProject())
    renderDetail()
    await screen.findByRole('heading', { name: 'Лестница на второй этаж' })

    expect(screen.getByLabelText(/Шаг комфорта/)).toBeInTheDocument()
    expect(screen.queryByLabelText(/Наружный радиус R/)).not.toBeInTheDocument()
    expect(screen.queryByLabelText(/Ширина площадки Wp/)).not.toBeInTheDocument()
  })
})
// ---- Живая валидация при вводе в редакторе проекта (S-P5) ----
// PascalCase — клиентский transport преобразует ответы API в формате
// ValidationResult (snake_case на проводе → CamelCase в DTO).
// Спиральный марш отключён (S-152), поэтому блокирующий сценарий живой
// валидации проверяем на прямом марше: проступь вне нормы + готовое
// предложение «Применить».
const liveBlockedSpiral = {
  Valid: false,
  Blocking: true,
  Issues: [
    {
      Code: 'GEO-TREAD',
      Severity: 'error',
      Element: 'tread_depth',
      Message: 'проступь вне нормы',
      Param: 'Шаг комфорта',
      Guide: 'Проступь 240 мм вне диапазона 260–320 мм.',
      Suggestions: [
        {
          StepCount: 18,
          StepHeightMm: 150,
          TreadDepthMm: 290,
          AngleDeg: 31.3,
          OuterRadiusMm: 1050,
          WidthMm: 900,
        },
      ],
    },
  ],
} as unknown as Awaited<ReturnType<typeof projectsApi.validateStair>>

describe('ProjectDetail · живая валидация (S-P5)', () => {
  it('вызывает :validate при изменении полей, показывает баннер и «Применить» пересчитывает', async () => {
    vi.spyOn(projectsApi, 'get').mockResolvedValue(makeProject())
    const validate = vi.spyOn(projectsApi, 'validateStair').mockResolvedValue(liveBlockedSpiral)
    const calculate = vi.spyOn(projectsApi, 'calculate').mockResolvedValue(makeCalculation())
    renderDetail()

    await screen.findByRole('heading', { name: 'Лестница на второй этаж' })
    fireEvent.change(screen.getByLabelText(/Шаг комфорта/), { target: { value: '600' } })

    await waitFor(() => expect(validate).toHaveBeenCalledTimes(1), { timeout: 2500 })
    const body = validate.mock.calls[0][0] as Record<string, unknown>
    expect(body.flight).toBe('straight')
    expect(body.comfort_step_mm).toBe(600)

    // Guide появляется и как ошибка поля, и в баннере блокировки.
    await waitFor(() => {
      expect(
        screen.getAllByText(/Проступь 240 мм вне диапазона/).length,
      ).toBeGreaterThanOrEqual(2)
    }, { timeout: 2500 })

    // «Применить» из баннера: подставляем вариант и запускаем расчёт проекта.
    fireEvent.click(screen.getByText(/Применить: 18 ступ/))
    expect(await screen.findByText(/Расчёт сохранён/)).toBeInTheDocument()
    const [calcId, calcBody] = calculate.mock.calls[0] as [string, Record<string, unknown>]
    expect(calcId).toBe('p1')
    // «Применить» подставляет предложение советника (проступь 290 → шаг 640).
    expect(calcBody.comfort_step_mm).toBe(600)
  })
})

describe('ProjectDetail · высота марша до 3D (стены)', () => {
  it('высота из поля «Высота подъёма H, мм» доходит до вьювера и пересчёт меняет её', async () => {
    const result = makeCalculation({
      result: makeSnapshot({ mesh: mesh3d, room_mesh: mesh3d }),
    })
    vi.spyOn(projectsApi, 'get').mockResolvedValue(makeProject())
    const calculate = vi.spyOn(projectsApi, 'calculate').mockResolvedValue(result)
    renderDetail()

    await screen.findByRole('heading', { name: 'Лестница на второй этаж' })
    fireEvent.click(screen.getByRole('button', { name: 'Рассчитать' }))
    await screen.findByText(/Расчёт сохранён/)

    // Дефолт 2700 доходит до вьювера вместе с габаритами помещения.
    fireEvent.click(screen.getByRole('tab', { name: '3D' }))
    await waitFor(() => expect(viewerProps.current.heightMM).toBe(2700))
    expect(viewerProps.current).toMatchObject({ flight: 'straight', roomWidth: 3000, roomLength: 4200 })

    // Правка высоты и пересчёт — вьювер получает новое значение.
    fireEvent.change(screen.getByLabelText(/Высота подъёма H/), { target: { value: '2750' } })
    fireEvent.click(screen.getByRole('button', { name: 'Рассчитать' }))
    await screen.findByText(/Расчёт сохранён/)
    fireEvent.click(screen.getByRole('tab', { name: '3D' }))
    await waitFor(() => expect(viewerProps.current.heightMM).toBe(2750))
    expect(viewerProps.current.flight).toBe('straight')
    expect(calculate.mock.calls[0][1] as Record<string, unknown>).toMatchObject({ height_mm: 2700 })
    expect(calculate.mock.calls[1][1] as Record<string, unknown>).toMatchObject({ height_mm: 2750 })
  })
})
