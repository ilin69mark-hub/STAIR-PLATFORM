# STAIR PLATFORM — админ-панель (frontend): сборка и статическая раздача.
# Multi-stage: node build → nginx. `/api` проксируется на Go API.
# Структура сборки повторяет локальную: admin = frontend, shared = frontend/shared,
# чтобы относительный алиас @shared и резолв react (подъём node_modules от shared)
# совпадали с локальной разработкой.
# Stage 1: build
FROM node:22-alpine AS build
WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend ./
# Dependencies для shared-модуля: нода поднимается по дереву от src/shared и ищет
# node_modules в общем предке (локально это frontend-store/node_modules).
RUN mkdir -p ../frontend-store && ln -s /app/frontend/node_modules ../frontend-store/node_modules
COPY frontend/shared ../frontend/shared
RUN npm run build

# Stage 2: nginx
FROM nginx:1.27-alpine
COPY deployments/nginx/admin.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/frontend/dist /usr/share/nginx/html
EXPOSE 80
