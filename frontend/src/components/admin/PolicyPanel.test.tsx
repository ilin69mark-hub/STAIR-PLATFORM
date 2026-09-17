// PolicyPanel — presentational: поля политики безопасности + submit.
import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import type { AdminPolicy } from '@shared/types'
import { PolicyPanel } from './PolicyPanel'

const policy: AdminPolicy = {
  min_password_length: 8,
  require_number: false,
  require_upper: false,
  session_ttl_seconds: 86400,
  login_rate_limit_per_min: 10,
}

function setup() {
  const onChange = vi.fn()
  const onSave = vi.fn()
  render(<PolicyPanel policy={policy} onChange={onChange} onSave={onSave} />)
  return { onChange, onSave }
}

describe('PolicyPanel', () => {
  it('рендерит все поля с текущими значениями политики', () => {
    setup()
    expect(screen.getByLabelText('Минимальная длина пароля')).toHaveValue(8)
    expect(screen.getByLabelText('TTL сессии (секунды)')).toHaveValue(86400)
    expect(screen.getByLabelText('Лимит входа (попыток/мин)')).toHaveValue(10)
    expect(screen.getByLabelText('Требовать цифру в пароле')).not.toBeChecked()
    expect(screen.getByLabelText('Требовать заглавную букву')).not.toBeChecked()
  })

  it('изменение числовых полей вызывает onChange с числом', () => {
    const { onChange } = setup()
    fireEvent.change(screen.getByLabelText('Минимальная длина пароля'), { target: { value: '12' } })
    expect(onChange).toHaveBeenCalledWith('min_password_length', 12)
    fireEvent.change(screen.getByLabelText('TTL сессии (секунды)'), { target: { value: '14400' } })
    expect(onChange).toHaveBeenCalledWith('session_ttl_seconds', 14400)
    fireEvent.change(screen.getByLabelText('Лимит входа (попыток/мин)'), { target: { value: '5' } })
    expect(onChange).toHaveBeenCalledWith('login_rate_limit_per_min', 5)
  })

  it('переключение чекбоксов вызывает onChange с булевым значением', () => {
    const { onChange } = setup()
    fireEvent.click(screen.getByLabelText('Требовать цифру в пароле'))
    expect(onChange).toHaveBeenCalledWith('require_number', true)
    fireEvent.click(screen.getByLabelText('Требовать заглавную букву'))
    expect(onChange).toHaveBeenCalledWith('require_upper', true)
  })

  it('сабмит формы вызывает onSave', () => {
    const { onSave } = setup()
    const form = screen.getByRole('button', { name: 'Сохранить политику' }).closest('form')
    expect(form).not.toBeNull()
    fireEvent.submit(form as HTMLFormElement)
    expect(onSave).toHaveBeenCalledTimes(1)
  })
})