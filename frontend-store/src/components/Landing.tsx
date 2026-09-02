import { useEffect, useState, type FormEvent } from 'react'
import { consultationsApi, testimonialsApi } from '../api/store'
import { apiErrorMessage } from '../auth/errors'
import type { TestimonialDTO } from '@shared/types'
import { HeroScene } from './HeroScene'

interface Props {
  onStart: () => void
}

// Лендинг: краткое описание продукта, призыв к расчёту, контакты, отзывы
// клиентов и форма запроса консультации.
export function Landing({ onStart }: Props) {
  const [testimonials, setTestimonials] = useState<TestimonialDTO[]>([])
  const [testimonialError, setTestimonialError] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false
    testimonialsApi
      .published()
      .then((items) => {
        if (!cancelled) setTestimonials(items)
      })
      .catch(() => {
        if (!cancelled) setTestimonialError('Не удалось загрузить отзывы')
      })
    return () => {
      cancelled = true
    }
  }, [])

  return (
    <div>
      <section className="hero">
        <h1>Лестницы на заказ по вашим размерам</h1>
        <p>
          Укажите проём, высоту и тип марша — мгновенно получите геометрию и предварительную
          стоимость. Изготовление из стали, алюминия и дерева.
        </p>
        <div className="hero-actions">
          <button className="sp-btn sp-btn--primary" onClick={onStart}>
            Рассчитать стоимость
          </button>
        </div>
        <HeroScene />
      </section>

      <section className="features">
        <div className="feature">
          <h3>Прямые марши</h3>
          <p>Классические лестницы с наклонными маршами и комфортной геометрией ступеней.</p>
        </div>
        <div className="feature">
          <h3>С площадкой</h3>
          <p>L- и П-образные конструкции для проёмов с поворотом. Промежуточная площадка.</p>
        </div>
        <div className="feature">
          <h3>Спиральные</h3>
          <p>Винтовые лестницы с центральной колонной — компактное решение для стеснённых мест.</p>
        </div>
        <div className="feature">
          <h3>Прозрачная цена</h3>
          <p>Материалы, обработка и монтаж в детальном расчёте. Согласуем проект перед запуском.</p>
        </div>
      </section>

      <Testimonials items={testimonials} error={testimonialError} />

      <Consultation />

      <About />
    </div>
  )
}

// Testimonials — опубликованные отзывы клиентов (из админки).
function Testimonials({ items, error }: { items: TestimonialDTO[]; error: string | null }) {
  if (error) {
    return (
      <section className="testimonials">
        <p className="sub" style={{ textAlign: 'center', opacity: 0.6 }}>{error}</p>
      </section>
    )
  }
  if (items.length === 0) {
    return null
  }
  return (
    <section className="testimonials">
      <h2>Отзывы клиентов</h2>
      <div className="testimonials-grid">
        {items.map((t) => (
          <article className="testimonial" key={t.id}>
            <div className="testimonial-rating" aria-label={`Оценка ${t.rating} из 5`}>
              {'★'.repeat(t.rating)}
              <span className="empty">{'★'.repeat(5 - t.rating)}</span>
            </div>
            <p className="testimonial-text">{t.text}</p>
            <p className="testimonial-author">{t.author}</p>
          </article>
        ))}
      </div>
    </section>
  )
}

// Consultation — форма запроса консультации (анонимная, создаёт заказ-лид).
function Consultation() {  const [name, setName] = useState('')
  const [phone, setPhone] = useState('')
  const [email, setEmail] = useState('')
  const [question, setQuestion] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [done, setDone] = useState(false)
  const [busy, setBusy] = useState(false)

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault()
    if (!name.trim() || !email.trim() || !question.trim()) {
      setError('Укажите имя, email и вопрос')
      return
    }
    setBusy(true)
    setError(null)
    try {
      await consultationsApi.create({
        contact: { name: name.trim(), email: email.trim(), phone: phone.trim() },
        question: question.trim(),
      })
      setDone(true)
    } catch (err) {
      setError(apiErrorMessage(err, 'Не удалось отправить заявку'))
    } finally {
      setBusy(false)
    }
  }

  return (
    <section className="consultation">
      <h2>Получить консультацию</h2>
      <p className="sub">
        Оставьте контакты и вопрос — менеджер перезвонит и поможет выбрать лестницу.
      </p>
      {done ? (
        <div className="alert alert--ok">Заявка отправлена. Мы свяжемся с вами в ближайшее время.</div>
      ) : (
        <form className="consultation-form" onSubmit={handleSubmit}>
          <div className="field">
            <label htmlFor="cons-name">Имя</label>
            <input id="cons-name" type="text" autoComplete="name" value={name} onChange={(e) => setName(e.target.value)} />
          </div>
          <div className="field">
            <label htmlFor="cons-phone">Телефон</label>
            <input id="cons-phone" type="tel" autoComplete="tel" placeholder="+7 (___) ___-__-__" value={phone} onChange={(e) => setPhone(e.target.value)} />
          </div>
          <div className="field">
            <label htmlFor="cons-email">Email</label>
            <input id="cons-email" type="email" autoComplete="email" value={email} onChange={(e) => setEmail(e.target.value)} />
          </div>
          <div className="field">
            <label htmlFor="cons-question">Ваш вопрос</label>
            <textarea id="cons-question" rows={3} value={question} onChange={(e) => setQuestion(e.target.value)} />
          </div>
          {error && <div className="alert alert--error">{error}</div>}
          <div className="actions">
            <button className="sp-btn sp-btn--primary" type="submit" disabled={busy}>
              {busy ? 'Отправка…' : 'Отправить заявку'}
            </button>
          </div>
        </form>
      )}
    </section>
  )
}

// About — блок «О нас» под формой консультации.
function About() {
  return (
    <section className="about">
      <h2>О нас</h2>
      <p className="sub">
        STAIR PLATFORM — собственное производство лестниц в Москве и области. Проектируем,
        изготавливаем и монтируем под ключ.
      </p>
      <div className="about-grid">
        <div className="about-item">
          <h3>Собственное производство</h3>
          <p>
            Цех площадью более 500 м²: резка, сварка, покраска и сборка. Контролируем каждый
            этап без посредников.
          </p>
        </div>
        <div className="about-item">
          <h3>Инженерный расчёт</h3>
          <p>
            Каждый проект просчитывается по нагрузкам, прочности и эргономике — сходите по
            лестнице так же комфортно, как и по проекту.
          </p>
        </div>
        <div className="about-item">
          <h3>Сроки и гарантия</h3>
          <p>
            Изготовление от 2 недель. Даём гарантию на конструкцию и окраску, а монтаж
            выполняют наши бригады.
          </p>
        </div>
      </div>
    </section>
  )
}