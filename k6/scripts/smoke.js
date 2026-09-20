// k6 (grafana/k6). PR-смок (S5-2) + ночной нагрузочный прогон (P3-16):
// дымовой сценарий против локального CI-API / staging. Цель — поймать
// регрессии производительности, не перегружая окружение.
//
// Запуск локально:
//   k6 run --env BASE_URL=http://localhost:8080 k6/scripts/smoke.js
//
// Пороговые значения (error budget INFRA-0022, S5-2):
//   - no failed requests (abortOnFail — ред фолд сразу);
//   - p95 валидации (паблик-расчёт) < 500ms;
//   - p95 всех запросов (health + validate) < 1000ms.
import http from "k6/http";
import { check, sleep } from "k6";

export const options = {
  stages: [
    { duration: "30s", target: 5 }, // разогрев
    { duration: "2m", target: 20 }, // номинальная нагрузка
    { duration: "30s", target: 0 }, // спад
  ],
  thresholds: {
    http_req_failed: [{ threshold: "rate<0.01", abortOnFail: true }],
    http_req_duration: [{ threshold: "p(95)<1000" }],
    "http_req_duration{name:validate}": [{ threshold: "p(95)<500" }],
  },
  summaryTrendStats: ["avg", "min", "med", "p(95)", "p(99)", "max"],
};

const BASE_URL = __ENV.BASE_URL || "http://localhost:8080";

// Минимально валидные параметры лестницы (прямой марш).
const PARAMS = {
  format: "direct",
  height: 3000,
  depth: 300,
  width: 1000,
  stepHeight: 175,
  treadDepth: 280,
  stringerThickness: 10,
  material: "STEEL-S235",
};

export default function () {
  // Живучесть: почти нулевая цена. GET /health (router.go).
  const health = http.get(`${BASE_URL}/health`);
  check(health, { "health 200": (r) => r.status === 200 });

  // Валидация параметров (без авторизации, лимит validate_rate_limit).
  // name-таг «validate» — отдельный порог p95<500ms (S5-2).
  const valid = http.post(
    `${BASE_URL}/api/v1/public/stairs:validate`,
    JSON.stringify(PARAMS),
    {
      headers: { "Content-Type": "application/json" },
      tags: { name: "validate" },
    },
  );
  check(valid, {
    "validate 200": (r) => r.status === 200,
  });

  sleep(0.1);
}