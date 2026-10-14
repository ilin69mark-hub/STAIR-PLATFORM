// Удаление сотрудника в плитке «Команда».
//
// Проверяем то, на что смотрит человек перед нажатием: кнопка есть только при
// staff.delete, перед удалением показаны точные цифры (сколько проектов
// уедет, сколько останется в истории), и удаление нельзя подтвердить, пока
// не выбран получатель проектов.
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import type { AdminUser } from '@shared/types'

import { UsersPanel } from './UsersPanel'

const manager: AdminUser = {
  id: 'u-1',
  email: 'manager@example.com',
  name: 'Менеджер',
  role: 'user',
  status: 'active',
  tenant_id: 't-1',
}

const owner: AdminUser = {
  id: 'u-0',
  email: 'owner@example.com',
  name: 'Владелец',
  role: 'owner',
  status: 'active',
  tenant_id: 't-1',
}

const blocked: AdminUser = {
  id: 'u-2',
  email: 'blocked@example.com',
  name: 'Заблокированный',
  role: 'user',
  status: 'disabled',
  tenant_id: 't-1',
}

const preview = {
  projects: 3,
  calculations: 12,
  comments: 5,
  orders: 2,
  project_roles: 1,
}

function setup({ canDelete = true, onDelete = vi.fn(async () => {}) } = {}) {
  const onPreviewDelete = vi.fn(async () => preview)
  render(
    <UsersPanel
      users={[owner, manager, blocked]}
      currentUserId={owner.id}
      onUpdate={vi.fn()}
      canDelete={canDelete}
      onPreviewDelete={onPreviewDelete}
      onDelete={onDelete}
    />,
  )
  return { onDelete, onPreviewDelete }
}

describe('удаление сотрудника', () => {
  it('без staff.delete кнопки удаления нет', () => {
    setup({ canDelete: false })
    expect(screen.queryByRole('button', { name: 'Удалить' })).not.toBeInTheDocument()
  })

  it('у себя кнопки удаления нет: сервер всё равно запретит', () => {
    // canDelete = true, но currentUserId = owner — у своей строки кнопки нет.
    setup({ canDelete: true })
    expect(screen.getAllByRole('button', { name: 'Удалить' })).toHaveLength(2)
    expect(screen.getAllByRole('button', { name: 'Удалить' })[0]).toBeInTheDocument()
  })

  it('сначала показывает точные цифры из сервера', async () => {
    const { onPreviewDelete } = setup()
    fireEvent.click(screen.getAllByRole('button', { name: 'Удалить' })[0])

    await waitFor(() =>
      expect(screen.getByRole('group', { name: `Удаление ${manager.email}` })).toBeInTheDocument(),
    )
    expect(onPreviewDelete).toHaveBeenCalledWith(manager.id)
    // Цифры приходят с сервера, а не склеиваются на клиенте.
    expect(screen.getByText(/проект\(ов\)/)).toBeInTheDocument()
    expect(screen.getByText(/расчётов — 12/)).toBeInTheDocument()
    expect(screen.getByText(/заказов — 2/)).toBeInTheDocument()
  })

  it('до выбора получателя удаление нельзя подтвердить', async () => {
    const { onDelete } = setup()
    fireEvent.click(screen.getAllByRole('button', { name: 'Удалить' })[0])

    const confirm = await screen.findByRole('button', { name: 'Передать и удалить' })
    expect(confirm).toBeDisabled()

    fireEvent.click(confirm)
    expect(onDelete).not.toHaveBeenCalled()
  })

  it('передаёт удаление с получателем проектов', async () => {
    const { onDelete } = setup()
    fireEvent.click(screen.getAllByRole('button', { name: 'Удалить' })[0])

    const select = await screen.findByLabelText(`Передать проекты ${manager.email}`)
    // Заблокированный сотрудник получателем быть не может — проект уехал бы
    // к тому, кто им не сможет пользоваться.
    expect(screen.queryByRole('option', { name: new RegExp(blocked.email) })).not.toBeInTheDocument()
    expect(screen.getByRole('option', { name: new RegExp(owner.email) })).toBeInTheDocument()

    fireEvent.change(select, { target: { value: owner.id } })
    fireEvent.click(screen.getByRole('button', { name: 'Передать и удалить' }))

    await waitFor(() => expect(onDelete).toHaveBeenCalledWith(manager.id, owner.id))
  })

  it('ошибка сервера показывается и панель не закрывается', async () => {
    const onDelete = vi.fn(async () => {
      throw new Error('Нельзя удалить последнего владельца компании.')
    })
    setup({ onDelete })
    fireEvent.click(screen.getAllByRole('button', { name: 'Удалить' })[0])

    const select = await screen.findByLabelText(`Передать проекты ${manager.email}`)
    fireEvent.change(select, { target: { value: owner.id } })
    fireEvent.click(screen.getByRole('button', { name: 'Передать и удалить' }))

    await waitFor(() =>
      expect(screen.getByRole('alert')).toHaveTextContent('Нельзя удалить последнего владельца компании.'),
    )
    // Панель осталась открытой, чтобы не заставлять повторять выбор.
    expect(screen.getByRole('group', { name: `Удаление ${manager.email}` })).toBeInTheDocument()
  })

  it('отмена закрывает панель без удаления', async () => {
    const { onDelete } = setup()
    fireEvent.click(screen.getAllByRole('button', { name: 'Удалить' })[0])
    fireEvent.click(await screen.findByRole('button', { name: 'Отмена' }))

    await waitFor(() =>
      expect(screen.queryByRole('group', { name: `Удаление ${manager.email}` })).not.toBeInTheDocument(),
    )
    expect(onDelete).not.toHaveBeenCalled()
  })
})
