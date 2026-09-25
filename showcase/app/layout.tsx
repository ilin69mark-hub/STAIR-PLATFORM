import type { Metadata } from 'next'
import Link from 'next/link'
import './globals.css'

export const metadata: Metadata = {
  metadataBase: new URL(process.env.SITE_URL ?? 'http://localhost:5176'),
  title: {
    default: 'Лестницы на заказ — расчёт, 3D и цена за минуту',
    template: '%s — лестницы на заказ',
  },
  description:
    'Проектируем лестницы для квартир и домов: честный расчёт геометрии, 3D-визуализация и предварительная цена по материалам. Прямые, L-образные, П-образные и спиральные марши.',
  openGraph: {
    type: 'website',
    locale: 'ru_RU',
    siteName: 'Лестницы на заказ',
  },
}

export default function RootLayout({ children }: LayoutProps<'/'>) {
  return (
    <html lang="ru">
      <body>
        <header className="site-header">
          <div className="wrap site-header__inner">
            <Link href="/" className="logo">
              Лестницы на заказ
            </Link>
            <nav className="nav">
              <Link href="/materials">Материалы</Link>
              <Link href="/examples">Примеры</Link>
              <Link href="/calculator">Калькулятор</Link>
              <Link href="/#faq">Вопросы</Link>
              <Link href="/calculator" className="btn">
                Рассчитать
              </Link>
            </nav>
          </div>
        </header>
        {children}
        <footer className="site-footer">
          <div className="wrap">
            <p>
              Расчёт носит предварительный характер: финальная стоимость
              зависит от замеров, доставки и монтажа. Цены материалов в каталоге —
              ориентировочные.
            </p>
          </div>
        </footer>
      </body>
    </html>
  )
}
