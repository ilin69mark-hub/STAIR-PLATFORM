import { ServicesPay } from '../services-pay'

export const metadata = {
  title: 'Услуги',
  description:
    'Выезд инженера с замером, проект и рабочая документация лестницы. Цены из серверного каталога, оплата онлайн после входа.',
}

export default function ServicesPage() {
  return (
    <main className="wrap section">
      <h1>Услуги</h1>
      <p className="section__head">
        Замер и проект лестницы можно оплатить онлайн. Стоимость лестницы по
        результатам замера считается отдельно — здесь только инженерные услуги.
      </p>
      <div id="services">
        <ServicesPay />
      </div>
    </main>
  )
}
