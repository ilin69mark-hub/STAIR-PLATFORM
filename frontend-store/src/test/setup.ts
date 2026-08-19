import '@testing-library/jest-dom/vitest'
import { afterEach, beforeEach } from 'vitest'
import { cleanup } from '@testing-library/react'

// localStorage в jsdom 29 отсутствует — эмуляция Map.
const storage = new Map<string, string>()
;(globalThis as Record<string, unknown>).localStorage = {
  getItem: (k: string) => storage.get(k) ?? null,
  setItem: (k: string, v: string) => {
    storage.set(k, v)
  },
  removeItem: (k: string) => {
    storage.delete(k)
  },
  clear: () => storage.clear(),
  key: (i: number) => Array.from(storage.keys())[i] ?? null,
  get length() {
    return storage.size
  },
}

beforeEach(() => {
  storage.clear()
})

afterEach(() => {
  cleanup()
})