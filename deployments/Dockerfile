# STAIR PLATFORM — API image
# Build context: repo root (../ from deployments/). See docker-compose.yml.

FROM golang:1.26-alpine AS build
WORKDIR /src

# Информация о сборке (версия/коммит/время) подставляется через ldflags,
# значения по умолчанию — dev/none/unknown.
ARG VERSION=dev
ARG COMMIT=none
ARG BUILD_TIME=unknown

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath \
    -ldflags "-s -w \
      -X stairplatform/internal/version.Version=${VERSION} \
      -X stairplatform/internal/version.Commit=${COMMIT} \
      -X stairplatform/internal/version.BuildTime=${BUILD_TIME}" \
    -o /bin/api ./cmd/api
RUN CGO_ENABLED=0 go build -trimpath \
    -ldflags "-s -w \
      -X stairplatform/internal/version.Version=${VERSION} \
      -X stairplatform/internal/version.Commit=${COMMIT} \
      -X stairplatform/internal/version.BuildTime=${BUILD_TIME}" \
    -o /bin/worker ./cmd/worker

FROM alpine:3.20
RUN adduser -D -H -u 10001 appuser
COPY --from=build /bin/api /bin/api
COPY --from=build /bin/worker /bin/worker
COPY migrations /migrations
WORKDIR /
USER appuser
EXPOSE 8080
ENTRYPOINT ["/bin/api"]
