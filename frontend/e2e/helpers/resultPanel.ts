// E2E: панели результата (Геометрия/Производство/Стоимость) спрятаны за
// result-tabs (S-100). Перед проверкой содержимого панели кликаем её таб,
// затем ассертим заголовок панели. exact: true — таб «Производство»
// задублирован во вкладках AI-ассистента (AssistantPanel).
import { expect, type Page } from '@playwright/test'

export async function expectResultPanel(
  page: Page,
  tabName: string,
  headingName: string,
): Promise<void> {
  await page.getByRole('tab', { name: tabName, exact: true }).click()
  await expect(page.getByRole('heading', { name: headingName, exact: true })).toBeVisible()
}