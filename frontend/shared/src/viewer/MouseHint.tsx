/**
 * MouseHint — еле заметная расшифровка мыши над 3D-сценой.
 *
 * Зачем: после смены управления (левая кнопка двигает сцену, правая крутит)
 * прежняя подпись внизу вьюера врала, а новая без наглядности не читалась:
 * «ПКМ» в народе означает контекстное меню, и человек просто не понимает,
 * что сцена должна ехать. Рисунок мыши с зажатой кнопкой снимает вопрос.
 *
 * Намеренно «еле заметная» (решение владельца): стоит поверх полотна,
 * приглушена, ничем не перекрывает изделие и не перехватывает указатель —
 * подсказка не должна мешать смотреть и вращать сцену.
 */

/** Зажатая кнопка мыши. Контур корпуса, разделитель и залитая половина. */
function MouseGlyph({ side }: { side: 'left' | 'right' }) {
  return (
    <svg
      className="viewer__mouse-hint__glyph"
      viewBox="0 0 20 28"
      aria-hidden="true"
      focusable="false"
    >
      <rect
        x="1.2"
        y="1.2"
        width="17.6"
        height="25.6"
        rx="8.8"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.5"
      />
      <path
        d="M10 1.2v8.4"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.3"
      />
      {side === 'left' ? (
        <path d="M1.2 9.6A8.8 8.8 0 0 1 10 1.2v8.4z" fill="currentColor" />
      ) : (
        <path d="M18.8 9.6A8.8 8.8 0 0 0 10 1.2v8.4z" fill="currentColor" />
      )}
      <path
        d="M10 13v4.5"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.2"
        strokeLinecap="round"
      />
    </svg>
  )
}

export function MouseHint() {
  return (
    <div className="viewer__mouse-hint">
      <span className="viewer__mouse-hint__item">
        <MouseGlyph side="left" />
        <span className="viewer__mouse-hint__key">ЛКМ</span>
        <span>двигать сцену</span>
      </span>
      <span className="viewer__mouse-hint__item">
        <MouseGlyph side="right" />
        <span className="viewer__mouse-hint__key">ПКМ</span>
        <span>крутить</span>
      </span>
      <span className="viewer__mouse-hint__item">
        <span className="viewer__mouse-hint__key">колесо</span>
        <span>зум к курсору</span>
      </span>
    </div>
  )
}
