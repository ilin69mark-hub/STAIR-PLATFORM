# STAIR PLATFORM — клиентский сайт (store): сборка и статическая раздача.
# Multi-stage: node build → nginx. `/api` проксируется на Go API.
# Структура сборки повторяет локальную: store = frontend-store, shared = frontend/shared,
# чтобы относительный алиас @shared и резолв react (подъём node_modules от shared)
# совпадали с poetry-lock'ом локальной разработки.
# Stage 1: build
FROM node:22-alpine AS build
WORKDIR /app/frontend-store
COPY frontend-store/package*.json ./
RUN npm ci
COPY frontend-store ./
# Dependencies для shared-модуля: нода поднимается по дереву от src/shared и ищет
# node_modules в общем предке (локально это frontend/node_modules).
RUN mkdir -p ../frontend && ln -s /app/frontend-store/node_modules ../frontend/node_modules
COPY frontend/shared ../frontend/shared
RUN npm run build

# Stage 2: nginx
FROM nginx:1.27-alpine
COPY deployments/nginx/store.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/frontend-store/dist /usr/share/nginx/html
EXPOSE 80