import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { ApiError } from '@shared/types'
import { adminApi } from '../../api/admin'
import { ApiKeysPanel } from './ApiKeysPanel'

vi.mock('../../api/admin', () => ({
  adminApi: {
    createApiKey: vi.fn(),
    revokeApiKey: vi.fn(),
  },
}))

const createApiKey = vi.mocked(adminApi.createApiKey)
const revokeApiKey = vi.mocked(adminApi.revokeApiKey)

function key(over: Partial<Parameters<typeof ApiKeysPanel>[0]['keys'][number]> = {}) {
  return {
    id: 'k-1',
    name: 'CI',
    scopes: ['users.list'],
    revoked_at: null,
    created_at: '2026-10-01T10:00:00Z',
    last_used_at: null,
    ...over,
  } as Parameters<typeof ApiKeysPanel>[0]['keys'][number]
}

function setup(keys = [key()]) {
  const onRefresh = vi.fn()
  const onError = vi.fn()
  const onNotice = vi.fn()
  render(<ApiKeysPanel keys={keys} onRefresh={onRefresh} onError={onError} onNotice={onNotice} />)
  return { onRefresh, onError, onNotice }
}

beforeEach(() => {
  vi.clearAllMocks()
})

describe('ApiKeysPanel', () => {
  it('показывает пустое состояние без ключей', () => {
    setup([])
    expect(screen.getByText('API-ключей пока нет.')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Отозвать' })).not.toBeInTheDocument()
  })

  it('перечисляет ключ с областями и статусом', () => {
    setup([key()])
    expect(screen.getByText(/CI · users\.list/)).toBeInTheDocument()
    expect(screen.getByText('Активен')).toBeInTheDocument()
  })

  it('отмечает отозванный ключ и не даёт отозвать повторно', () => {
    // Повторный отзыв отозванного ключа бессмысленен и возвращает ошибку на
    // сервере: кнопка у такого ключа быть не должна.
    setup([key({ revoked_at: '2026-10-05T10:00:00Z' })])
    expect(screen.getByText('Отозван')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Отозвать' })).not.toBeInTheDocument()
  })

  it('создаёт ключ, разбирая scopes и очищая форму', async () => {
    const user = userEvent.setup()
    const { onRefresh } = setup([])
    createApiKey.mockResolvedValue({ token: 'tok-1' } as never)

    await user.type(screen.getByPlaceholderText('Имя ключа (например, CI)'), '  CI  ')
    await user.clear(screen.getByPlaceholderText('Scopes через запятую (users.list)'))
    await user.type(screen.getByPlaceholderText('Scopes через запятую (users.list)'), ' users.list , audit.read ,, ')
    await user.click(screen.getByRole('button', { name: 'Создать ключ' }))

    await waitFor(() => expect(createApiKey).toHaveBeenCalled())
    // Пробелы обрезаются, пустые элементы выбрасываются: иначе на сервер уехал
    // бы scope из одних пробелов и пустой строки.
    expect(createApiKey).toHaveBeenCalledWith({ name: 'CI', scopes: ['users.list', 'audit.read'] })
    // Имя обрезается: « CI » и «CI» — один и тот же ключ в реестре.
    expect(onRefresh).toHaveBeenCalled()
    expect(screen.getByPlaceholderText('Имя ключа (например, CI)')).toHaveValue('')
  })

  it('показывает токен один раз, с предупреждением', async () => {
    const user = userEvent.setup()
    setup([])
    createApiKey.mockResolvedValue({ token: 'tok-secret' } as never)

    await user.type(screen.getByPlaceholderText('Имя ключа (например, CI)'), 'CI')
    await user.click(screen.getByRole('button', { name: 'Создать ключ' }))

    // Токен сервер отдаёт один раз и больше не вернёт: если не показать его
    // сразу, ключ придётся перевыпускать.
    await waitFor(() => expect(screen.getByText('tok-secret')).toBeInTheDocument())
    // Фраза встречается и в подписи панели, и в самом предупреждении — берём
    // именно его, иначе getByText находит оба и падает.
    expect(screen.getByText(/показывается один раз:$/)).toBeInTheDocument()
  })

  it('не показывает блок токена, если сервер его не вернул', async () => {
    const user = userEvent.setup()
    setup([])
    createApiKey.mockResolvedValue({} as never)

    await user.type(screen.getByPlaceholderText('Имя ключа (например, CI)'), 'CI')
    await user.click(screen.getByRole('button', { name: 'Создать ключ' }))

    await waitFor(() => expect(createApiKey).toHaveBeenCalled())
    // Пустой блок «Сохраните токен» с пустым значением только пугает.
    expect(screen.queryByText('Сохраните токен сейчас')).not.toBeInTheDocument()
  })

  it('показывает текст ошибки ApiError при создании', async () => {
    const user = userEvent.setup()
    const { onError } = setup([])
    createApiKey.mockRejectedValue(new ApiError(403, 'forbidden', 'не хватает прав'))

    await user.type(screen.getByPlaceholderText('Имя ключа (например, CI)'), 'CI')
    await user.click(screen.getByRole('button', { name: 'Создать ключ' }))

    await waitFor(() => expect(onError).toHaveBeenCalledWith('не хватает прав'))
  })

  it('подставляет свой текст на не-API ошибке', async () => {
    const user = userEvent.setup()
    const { onError } = setup([])
    createApiKey.mockRejectedValue(new Error('обрыв сети'))

    await user.type(screen.getByPlaceholderText('Имя ключа (например, CI)'), 'CI')
    await user.click(screen.getByRole('button', { name: 'Создать ключ' }))

    await waitFor(() => expect(onError).toHaveBeenCalledWith('Не удалось создать API-ключ'))
  })

  it('отзывает ключ и подтверждает это', async () => {
    const user = userEvent.setup()
    const { onNotice, onRefresh } = setup([key()])
    revokeApiKey.mockResolvedValue(undefined as never)

    await user.click(screen.getByRole('button', { name: 'Отозвать' }))

    await waitFor(() => expect(revokeApiKey).toHaveBeenCalledWith('k-1'))
    expect(onNotice).toHaveBeenCalledWith('API-ключ отозван')
    expect(onRefresh).toHaveBeenCalled()
  })

  it('сообщает об ошибке отзыва', async () => {
    const user = userEvent.setup()
    const { onError, onNotice } = setup([key()])
    revokeApiKey.mockRejectedValue(new Error('нет'))

    await user.click(screen.getByRole('button', { name: 'Отозвать' }))

    await waitFor(() => expect(onError).toHaveBeenCalledWith('Не удалось отозвать ключ'))
    // Успех на отзыве не показываем, если отзыв не состоялся.
    expect(onNotice).not.toHaveBeenCalled()
  })
})