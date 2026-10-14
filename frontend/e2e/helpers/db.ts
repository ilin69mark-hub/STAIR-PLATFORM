// E2E: прямой доступ к БД через psql. Роль admin назначается только через БД/seed
// (SEC-0004), поэтому e2e повышает зарегистрированного пользователя здесь.
//
// Команда psql настраивается через STAIR_E2E_PSQL:
// локально — `docker exec stair-platform-postgres psql -U stair -d stair_platform`,
// в CI — `PGPASSWORD=testpassword psql -h localhost -U stair -d stair_platform_test`.
import { execSync } from 'node:child_process'

export function psql(sql: string): string {
  const base = process.env.STAIR_E2E_PSQL ?? 'docker exec stair-platform-postgres psql -U stair -d stair_platform'
  return execSync(`${base} -tAc "${sql}"`, { encoding: 'utf8' }).trim()
}

export function promoteToAdmin(email: string): void {
  const out = psql(`UPDATE users SET role='admin' WHERE email='${email}' AND role='user' RETURNING id`)
  if (!out) throw new Error(`Пользователь ${email} не найден — не удалось повысить права`)
}