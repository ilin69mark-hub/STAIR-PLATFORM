// Общие мелочи разделов панели магазина: загрузка и ошибка.
//
// Лежали в PricesSettings.tsx, откуда их импортировали три файла. После
// разделения настроек на группы по плиткам PricesSettings остался только
// ценами, и держать в нём чужие компоненты было неправильно: файл называется
// по одной секции, а содержал общие для всех.
export function Loading() {
  return <p className="muted">Загрузка…</p>
}

export function ErrorBox({ text }: { text: string }) {
  return (
    <p className="alert alert--error" role="alert">
      {text}
    </p>
  )
}