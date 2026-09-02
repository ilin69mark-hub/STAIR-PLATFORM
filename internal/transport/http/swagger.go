package http

import (
	"embed"
	"html/template"
	"net/http"
)

//go:embed swagger/*
var swaggerFS embed.FS

// SwaggerPage — HTML страница для Swagger UI.
const SwaggerPage = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>STAIR Platform API - Swagger UI</title>
    <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.10.5/swagger-ui.css">
</head>
<body>
    <div id="swagger-ui"></div>
    <script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.10.5/swagger-ui-bundle.js"></script>
    <script>
        SwaggerUIBundle({
            url: "/docs/openapi/swagger.yaml",
            dom_id: '#swagger-ui',
            presets: [
                SwaggerUIBundle.presets.apis,
                SwaggerUIBundle.SwaggerUIStandalonePreset
            ],
            layout: "BaseLayout"
        });
    </script>
</body>
</html>`

// handleSwaggerUI — GET /swagger (interactive API documentation).
func handleSwaggerUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl, err := template.New("swagger").Parse(SwaggerPage)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	tmpl.Execute(w, nil)
}

// handleSwaggerSpec — GET /docs/openapi/swagger.yaml (OpenAPI spec).
func handleSwaggerSpec(w http.ResponseWriter, r *http.Request) {
	data, err := swaggerFS.ReadFile("swagger/swagger.yaml")
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/yaml")
	w.Write(data)
}
