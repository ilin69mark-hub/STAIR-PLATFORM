package http

import (
	"encoding/json"
	"net/http"
)

// NewRouter собирает маршруты API v1. svc — прикладной сервис расчёта
// (инверсия зависимостей, DOM-0008); nil недопустим. Роутер оборачивает
// все маршруты middleware логгирования/request-id (наблюдаемость).
func NewRouter(svc StairService) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handleHealth)
	mux.HandleFunc("POST /api/v1/stairs:calculate", handleCalculate(svc))
	mux.HandleFunc("GET /", handleNotFound)
	return withLogging(mux)
}

func handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "stair-platform-api",
	})
}

func handleNotFound(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusNotFound, map[string]string{
		"error": "not_found",
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
