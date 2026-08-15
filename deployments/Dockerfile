# STAIR PLATFORM — API image
# Build context: repo root (../ from deployments/). See docker-compose.yml.

FROM golang:1.26-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /bin/api ./cmd/api
RUN CGO_ENABLED=0 go build -trimpath -o /bin/worker ./cmd/worker

FROM alpine:3.20
RUN adduser -D -H -u 10001 appuser
COPY --from=build /bin/api /bin/api
COPY --from=build /bin/worker /bin/worker
COPY migrations /migrations
WORKDIR /
USER appuser
EXPOSE 8080
ENTRYPOINT ["/bin/api"]
