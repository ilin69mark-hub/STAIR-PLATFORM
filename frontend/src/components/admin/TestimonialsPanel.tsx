import React, { useState } from 'react'
import type { TestimonialDTO } from '@shared/types'
import { ApiError } from '@shared/types'
import { adminApi } from '../../api/admin'

interface Props {
  testimonials: TestimonialDTO[]
  onRefresh: () => void
  onError: (msg: string) => void
  onNotice: (msg: string) => void
}

export const TestimonialsPanel = React.memo(function TestimonialsPanel({
  testimonials,
  onRefresh,
  onError,
  onNotice,
}: Props) {
  const [tAuthor, setTAuthor] = useState('')
  const [tText, setTText] = useState('')
  const [tRating, setTRating] = useState(5)

  const handleAdd = async (e: React.FormEvent) => {
    e.preventDefault()
    onError('')
    try {
      await adminApi.createTestimonial({
        author: tAuthor.trim(),
        text: tText.trim(),
        rating: tRating,
      })
      setTAuthor('')
      setTText('')
      setTRating(5)
      onNotice('Отзыв добавлен. Опубликуйте его для лендинга.')
      onRefresh()
    } catch (err) {
      onError(err instanceof ApiError ? err.message : 'Не удалось добавить отзыв')
    }
  }

  const handleToggle = async (t: TestimonialDTO) => {
    onError('')
    try {
      await adminApi.updateTestimonial(t.id, {
        author: t.author,
        text: t.text,
        rating: t.rating,
        published: !t.published,
      })
      onNotice(t.published ? 'Отзыв скрыт с лендинга' : 'Отзыв опубликован на лендинге')
      onRefresh()
    } catch (err) {
      onError(err instanceof ApiError ? err.message : 'Не удалось обновить отзыв')
    }
  }

  const handleDelete = async (id: string) => {
    onError('')
    try {
      await adminApi.deleteTestimonial(id)
      onNotice('Отзыв удалён')
      onRefresh()
    } catch (err) {
      onError(err instanceof ApiError ? err.message : 'Не удалось удалить отзыв')
    }
  }

  return (
    <section className="panel">
      <h2 className="panel__title">Отзывы (store)</h2>
      <p className="muted">Отзывы клиентов на лендинге. Опубликованные видны всем посетителям.</p>
      {testimonials.length === 0 ? (
        <p className="muted">Отзывов пока нет. Добавьте первый ниже.</p>
      ) : (
        <ul className="comment-list">
          {testimonials.map((t) => (
            <li className="comment" key={t.id}>
              <div className="comment__meta">
                <span className="comment__author">
                  {t.author} · {'★'.repeat(t.rating)}
                  <span className="badge badge--sm">{t.published ? 'Опубликован' : 'Черновик'}</span>
                </span>
                <span className="comment__date">
                  {new Date(t.created_at).toLocaleString('ru-RU')}
                </span>
              </div>
              <p className="comment__body">{t.text}</p>
              <div className="comment__actions">
                <button className="btn btn--sm" onClick={() => void handleToggle(t)}>
                  {t.published ? 'Скрыть' : 'Опубликовать'}
                </button>
                <button
                  className="btn btn--danger btn--sm"
                  onClick={() => void handleDelete(t.id)}
                >
                  Удалить
                </button>
              </div>
            </li>
          ))}
        </ul>
      )}
      <form className="member-add" onSubmit={handleAdd}>
        <input
          className="field__input"
          placeholder="Автор (имя клиента)"
          value={tAuthor}
          onChange={(e) => setTAuthor(e.target.value)}
          required
        />
        <input
          className="field__input"
          placeholder="Текст отзыва"
          value={tText}
          onChange={(e) => setTText(e.target.value)}
          required
        />
        <select
          className="member__role"
          aria-label="Оценка отзыва"
          value={tRating}
          onChange={(e) => setTRating(Number(e.target.value))}
        >
          <option value={5}>5 ★</option>
          <option value={4}>4 ★</option>
          <option value={3}>3 ★</option>
          <option value={2}>2 ★</option>
          <option value={1}>1 ★</option>
        </select>
        <button className="btn btn--primary" type="submit">
          Добавить отзыв
        </button>
      </form>
    </section>
  )
})
