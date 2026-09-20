import { expect, test } from '@playwright/test'

// E2E S3-1 (SEO аудит-план): лендинг несёт уникальный title, meta-описание,
// Open Graph и структурированные данные Organization/LocalBusiness.
// Чисто клиентская проверка: API не требуется (статика + index.html).

const TITLE = 'STAIR PLATFORM — лестницы на заказ'

test('лендинг: уникальный title и meta-описание присутствуют', async ({ page }) => {
  await page.goto('/')
  await expect(page).toHaveTitle(TITLE)
  await expect(page.locator('meta[name="description"]')).toHaveAttribute(
    'content',
    /.+/,
  )
})

test('лендинг: Open Graph продублирован', async ({ page }) => {
  await page.goto('/')
  const ogTitle = page.locator('meta[property="og:title"]')
  await expect(ogTitle).toHaveCount(1)
  await expect(ogTitle).toHaveAttribute('content', TITLE)
  await expect(page.locator('meta[property="og:type"]')).toHaveAttribute(
    'content',
    'website',
  )
  await expect(page.locator('meta[property="og:locale"]')).toHaveAttribute(
    'content',
    'ru_RU',
  )
})

test('лендинг: JSON-LD Organization/LocalBusiness с контактами', async ({ page }) => {
  await page.goto('/')
  const jsonLd = await page
    .locator('script[type="application/ld+json"]')
    .textContent()
  expect(jsonLd).toBeTruthy()
  const parsed = JSON.parse(jsonLd ?? '{}') as {
    '@type'?: string | string[]
    name?: string
    email?: string
    address?: { addressLocality?: string }
  }
  const types = Array.isArray(parsed['@type'])
    ? parsed['@type'].join(',')
    : parsed['@type']
  expect(types).toContain('Organization')
  expect(types).toContain('LocalBusiness')
  expect(parsed.name).toBe('STAIR PLATFORM')
  expect(parsed.email).toBe('info@stair-platform.ru')
  expect(parsed.address?.addressLocality).toBe('Москва')
})

test('robots.txt доступен и разрешает индексацию', async ({ request }) => {
  const res = await request.get('/robots.txt')
  expect(res.status()).toBe(200)
  expect(await res.text()).toContain('Allow: /')
})