import { useEffect, useRef, useState } from 'react'
import './HeroScene.css'

// Геометрия марша (в координатах viewBox 960×420), восходящего слева направо.
const STEPS = 7
const TREAD = 72
const RISE = 36
const ORIGIN_X = 110
const FLOOR_Y = 360

// Центр (cx) и высота проступи (topY) i-й ступени — туда сажаем актёра.
function stepCenter(i: number) {
  const x0 = ORIGIN_X + TREAD * i
  const x1 = x0 + TREAD
  const topY = FLOOR_Y - RISE * (i + 1)
  return { cx: (x0 + x1) / 2, topY }
}

// Двое детей стоят рядом на ступени — смещаем их по горизонтали.
function kidCenter(step: number, lane: number) {
  const base = stepCenter(step)
  return { cx: base.cx + TREAD * 0.28 * lane, topY: base.topY }
}

function usePrefersReducedMotion() {
  const [reduced, setReduced] = useState(false)
  useEffect(() => {
    const mq = window.matchMedia?.('(prefers-reduced-motion: reduce)')
    if (!mq) {
      setReduced(false)
      return
    }
    setReduced(mq.matches)
    const handler = () => setReduced(mq.matches)
    mq.addEventListener?.('change', handler)
    return () => mq.removeEventListener?.('change', handler)
  }, [])
  return reduced
}

// Состояние одного персонажа. exit — уходит из кадра (вверх/вниз), hidden — спрятан.
interface ActorState {
  step: number
  sit: boolean
  dir: 'up' | 'down'
  exit?: 'up' | 'down'
  hidden?: boolean
  lane?: -1 | 1
}

// Кадр сцены: кот + двое детей.
interface Frame {
  cat: ActorState
  kids: [ActorState, ActorState]
  ms: number
}

// ---- Последовательность анимации (зацикленная смена кот ↔ дети) ----
const HOP = 720
const SIT_TOP = 1800
const SIT_BOTTOM = 1200
const ENTER = 1100
const EXIT = 900

const catParked: ActorState = { step: 0, sit: true, dir: 'up', hidden: true }
const kidsParked: [ActorState, ActorState] = [
  { step: 0, sit: true, dir: 'up', lane: -1, hidden: true },
  { step: 0, sit: true, dir: 'up', lane: 1, hidden: true },
]
const kid = (step: number, sit: boolean, lane: -1 | 1): ActorState => ({ step, sit, dir: 'up', lane })
const kidExit = (lane: -1 | 1): ActorState => ({ step: 0, sit: true, dir: 'up', lane, exit: 'down' })

function buildSeq(): Frame[] {
  const seq: Frame[] = []
  // Кот: вверх, сидит наверху, вниз, сидит внизу, упрыгивает за верхний край.
  for (let s = 0; s < STEPS; s++) seq.push({ cat: { step: s, sit: false, dir: 'up' }, kids: kidsParked, ms: HOP })
  seq.push({ cat: { step: STEPS - 1, sit: true, dir: 'up' }, kids: kidsParked, ms: SIT_TOP })
  for (let s = STEPS - 2; s >= 0; s--) seq.push({ cat: { step: s, sit: false, dir: 'down' }, kids: kidsParked, ms: HOP })
  seq.push({ cat: { step: 0, sit: true, dir: 'down' }, kids: kidsParked, ms: SIT_BOTTOM })
  seq.push({ cat: { step: STEPS - 1, sit: false, dir: 'up', exit: 'up' }, kids: kidsParked, ms: EXIT })
  // Дети: появляются внизу, лезут вверх, сидят, спускаются, уходят.
  seq.push({ cat: catParked, kids: [kid(0, true, -1), kid(0, true, 1)], ms: ENTER })
  for (let s = 0; s < STEPS; s++) seq.push({ cat: catParked, kids: [kid(s, false, -1), kid(s, false, 1)], ms: HOP })
  seq.push({ cat: catParked, kids: [kid(STEPS - 1, true, -1), kid(STEPS - 1, true, 1)], ms: SIT_TOP })
  for (let s = STEPS - 2; s >= 0; s--) seq.push({ cat: catParked, kids: [kid(s, false, -1), kid(s, false, 1)], ms: HOP })
  seq.push({ cat: catParked, kids: [kid(0, true, -1), kid(0, true, 1)], ms: SIT_BOTTOM })
  seq.push({ cat: catParked, kids: [kidExit(-1), kidExit(1)], ms: EXIT })
  return seq
}

const SEQ = buildSeq()

function staticFrame(): Frame {
  // Для prefers-reduced-motion: статичная сцена (кот сидит, дети стоят внизу).
  return {
    cat: { step: 3, sit: true, dir: 'up' },
    kids: [
      { step: 0, sit: true, dir: 'up', lane: -1 },
      { step: 0, sit: true, dir: 'up', lane: 1 },
    ],
    ms: 0,
  }
}

export function HeroScene() {
  const reduced = usePrefersReducedMotion()
  const [frame, setFrame] = useState<Frame>(() => (reduced ? staticFrame() : SEQ[0]))
  const timer = useRef<number | undefined>(undefined)

  useEffect(() => {
    if (reduced) {
      setFrame(staticFrame())
      return
    }
    let i = 0
    const tick = () => {
      const f = SEQ[i % SEQ.length]
      setFrame(f)
      i++
      timer.current = window.setTimeout(tick, f.ms)
    }
    timer.current = window.setTimeout(tick, 600)
    return () => window.clearTimeout(timer.current)
  }, [reduced])

  return (
    <div className="hero-scene" aria-hidden="true">
      <SceneSvg />
      <ActorView a={frame.cat} kind="cat" />
      <ActorView a={frame.kids[0]} kind="kid" tint="#5b8fd6" hair="#6b4a2f" />
      <ActorView a={frame.kids[1]} kind="kid" tint="#e08aa6" hair="#3a2c20" />
      <span className="hero-scene__sr">
        Мультяшная сцена: уютная лестница в профиль, по которой скачет кошка, а затем забираются двое детей.
      </span>
    </div>
  )
}

function posFor(a: ActorState, kind: 'cat' | 'kid') {
  if (a.exit === 'up') {
    const c = stepCenter(STEPS - 1)
    return { left: (c.cx / 960) * 100, top: -30 }
  }
  if (a.exit === 'down') {
    const c = stepCenter(0)
    return { left: (c.cx / 960) * 100, top: 122 }
  }
  const c = kind === 'cat' ? stepCenter(a.step) : kidCenter(a.step, a.lane ?? 0)
  return { left: (c.cx / 960) * 100, top: (c.topY / 420) * 100 }
}

function ActorView({
  a,
  kind,
  tint,
  hair,
}: {
  a: ActorState
  kind: 'cat' | 'kid'
  tint?: string
  hair?: string
}) {
  const pos = posFor(a, kind)
  const cls = [
    'actor',
    kind === 'cat' ? 'actor--cat' : 'actor--kid',
    a.hidden ? 'actor--hidden' : '',
    a.exit ? `actor--exit-${a.exit}` : '',
    a.sit ? 'actor--sit' : a.dir === 'up' ? 'actor--up' : 'actor--down',
  ]
    .filter(Boolean)
    .join(' ')
  const key = `${a.step}-${a.dir}-${a.sit}-${a.exit ?? ''}-${a.hidden ? 'h' : ''}`

  return (
    <div className={cls} style={{ left: `${pos.left}%`, top: `${pos.top}%` }}>
      <svg className="actor__art" viewBox="0 0 100 100" key={key}>
        {kind === 'cat' ? <CatPoses /> : <ChildPoses tint={tint ?? '#ccc'} hair={hair ?? '#999'} />}
      </svg>
    </div>
  )
}

// Позы кошки (плоский SVG, как раньше).
function CatPoses() {
  return (
    <>
      <g className="actor__pose actor__pose--hop">
        <path d="M62 70 C 88 60, 92 30, 78 20" fill="none" stroke="#6b4a2f" strokeWidth="7" strokeLinecap="round" />
        <ellipse cx="50" cy="62" rx="26" ry="22" fill="#caa06b" />
        <circle cx="40" cy="40" r="18" fill="#caa06b" />
        <path d="M26 30 L 30 12 L 42 26 Z" fill="#caa06b" />
        <path d="M50 26 L 56 10 L 60 30 Z" fill="#caa06b" />
        <rect x="36" y="78" width="7" height="14" rx="3" fill="#b98c52" />
        <rect x="56" y="78" width="7" height="14" rx="3" fill="#b98c52" />
        <circle cx="36" cy="40" r="3" fill="#3a2c20" />
        <path d="M40 46 l -4 4 l 8 0 Z" fill="#d98c86" />
      </g>
      <g className="actor__pose actor__pose--sit">
        <path d="M70 88 C 92 84, 94 60, 80 56 C 70 60, 66 76, 70 88 Z" fill="#6b4a2f" />
        <ellipse cx="50" cy="66" rx="24" ry="26" fill="#caa06b" />
        <circle cx="50" cy="40" r="19" fill="#caa06b" />
        <path d="M34 30 L 38 12 L 50 26 Z" fill="#caa06b" />
        <path d="M62 30 L 58 12 L 50 26 Z" fill="#caa06b" />
        <rect x="40" y="86" width="8" height="12" rx="3" fill="#b98c52" />
        <rect x="54" y="86" width="8" height="12" rx="3" fill="#b98c52" />
        <circle cx="44" cy="40" r="3.2" fill="#3a2c20" />
        <circle cx="56" cy="40" r="3.2" fill="#3a2c20" />
        <path d="M50 46 l -5 5 l 10 0 Z" fill="#d98c86" />
      </g>
    </>
  )
}

// Позы ребёнка (плоский SVG, в том же стиле, что и кот).
function ChildPoses({ tint, hair }: { tint: string; hair: string }) {
  return (
    <>
      <g className="actor__pose actor__pose--hop">
        <ellipse cx="50" cy="60" rx="19" ry="23" fill={tint} />
        <circle cx="50" cy="34" r="17" fill="#f3cda8" />
        <path d="M32 34 a 18 18 0 0 1 36 0 Z" fill={hair} />
        <rect x="44" y="84" width="7" height="13" rx="3" fill={tint} />
        <rect x="51" y="84" width="7" height="13" rx="3" fill={tint} />
        <circle cx="45" cy="35" r="2.6" fill="#3a2c20" />
        <circle cx="55" cy="35" r="2.6" fill="#3a2c20" />
        <path d="M45 41 q 5 4 10 0" stroke="#b5614f" strokeWidth="2" fill="none" strokeLinecap="round" />
      </g>
      <g className="actor__pose actor__pose--sit">
        <ellipse cx="50" cy="64" rx="19" ry="25" fill={tint} />
        <circle cx="50" cy="34" r="17" fill="#f3cda8" />
        <path d="M32 34 a 18 18 0 0 1 36 0 Z" fill={hair} />
        <rect x="42" y="86" width="7" height="12" rx="3" fill="#f3cda8" />
        <rect x="53" y="86" width="7" height="12" rx="3" fill="#f3cda8" />
        <circle cx="45" cy="35" r="2.6" fill="#3a2c20" />
        <circle cx="55" cy="35" r="2.6" fill="#3a2c20" />
        <path d="M45 41 q 5 4 10 0" stroke="#b5614f" strokeWidth="2" fill="none" strokeLinecap="round" />
      </g>
    </>
  )
}

function SceneSvg() {
  const steps = Array.from({ length: STEPS }, (_, i) => {
    const x0 = ORIGIN_X + TREAD * i
    const x1 = x0 + TREAD
    const topY = FLOOR_Y - RISE * (i + 1)
    return { x0, x1, topY }
  })

  // Поручень проходит ровно по верхушкам балясин (балясины высотой 64).
  const railY0 = steps[0].topY - 64
  const railY1 = steps[STEPS - 1].topY - 64

  return (
    <svg
      className="hero-scene__bg"
      viewBox="0 0 960 420"
      preserveAspectRatio="xMidYMid meet"
      role="presentation"
    >
      <defs>
        <linearGradient id="hs-wall" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0" stopColor="#fbe9cf" />
          <stop offset="1" stopColor="#f3d9b8" />
        </linearGradient>
        <linearGradient id="hs-wood" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0" stopColor="#c89255" />
          <stop offset="1" stopColor="#a9702f" />
        </linearGradient>
        <linearGradient id="hs-floor" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0" stopColor="#b07b43" />
          <stop offset="1" stopColor="#8a5e2c" />
        </linearGradient>
        <radialGradient id="hs-sun" cx="0.5" cy="0.5" r="0.5">
          <stop offset="0" stopColor="#fff3d6" />
          <stop offset="1" stopColor="#ffd98a" />
        </radialGradient>
      </defs>

      {/* стена */}
      <rect x="0" y="0" width="960" height="420" fill="url(#hs-wall)" />

      {/* окно с тёплым светом */}
      <rect x="700" y="50" width="200" height="150" rx="10" fill="#ffffff" opacity="0.5" />
      <rect x="700" y="50" width="200" height="150" rx="10" fill="url(#hs-sun)" />
      <line x1="800" y1="50" x2="800" y2="200" stroke="#caa56b" strokeWidth="6" />
      <line x1="700" y1="125" x2="900" y2="125" stroke="#caa56b" strokeWidth="6" />

      {/* пол */}
      <rect x="0" y="360" width="960" height="60" fill="url(#hs-floor)" />

      {/* марш лестницы (слева направо) */}
      {steps.map((s, i) => (
        <g key={i}>
          <rect x={s.x0} y={s.topY} width={TREAD} height={FLOOR_Y - s.topY} fill="url(#hs-wood)" />
          <rect x={s.x0} y={s.topY} width={TREAD} height="6" fill="#e0b878" />
        </g>
      ))}

      {/* перила + балясины */}
      <line
        x1={steps[0].x0 + TREAD / 2}
        y1={railY0}
        x2={steps[STEPS - 1].x0 + TREAD / 2}
        y2={railY1}
        stroke="#7c5230"
        strokeWidth="8"
        strokeLinecap="round"
      />
      {steps.map((s, i) => (
        <line
          key={i}
          x1={s.x0 + TREAD / 2}
          y1={s.topY}
          x2={s.x0 + TREAD / 2}
          y2={s.topY - 64}
          stroke="#9c6b3f"
          strokeWidth="5"
        />
      ))}

      {/* горшок с растением */}
      <rect x="840" y="300" width="70" height="70" rx="8" fill="#c97b4a" />
      <path d="M875 300 C 850 250, 860 230, 875 220 C 890 230, 900 250, 875 300 Z" fill="#5e8c4a" />
      <path d="M875 300 C 905 260, 920 250, 928 240 C 920 270, 905 290, 875 300 Z" fill="#6fa356" />
    </svg>
  )
}
