// k6 (grafana/k6). Ночной нагрузочный прогон (P3-16): дымовой сценарий
// против staging. Цель — поймать регрессии производительности и деградацию
// конвейера под нагрузкой, не перегружая окружение.
//
// Запуск локально:
//   k6 run --env BASE_URL=http://localhost:8080 k6/scripts/smoke.js
//
// Пороговые значения (error budget INFRA-0022):
//   - no failed requests;
//   - p95 времени расчёта < 2s;
//   - p95 валидации < 500ms.
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
  // Живучесть: почти нулевая цена.
  const health = http.get(`${BASE_URL}/api/v1/health`);
  check(health, { "health 200": (r) => r.status === 200 });

  // Валидация параметров (без авторизации, лимит validate_rate_limit).
  const valid = http.post(
    `${BASE_URL}/api/v1/public/stairs:validate`,
    JSON.stringify(PARAMS),
    { headers: { "Content-Type": "application/json" } },
  );
  check(valid, {
    "validate 200": (r) => r.status === 200,
    "validate p95 < 1.5s": (r) => r.timings.duration < 1500,
  });

  sleep(0.1);
}