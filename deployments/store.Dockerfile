# STAIR PLATFORM — клиентский сайт (store): сборка и статическая раздача.
# Multi-stage: node build → nginx. `/api` проксируется на Go API.
# Root npm workspaces (PR #36, S-117): единый package-lock.json на корне —
# зависимости всех воркспейсов ставятся одним `npm ci`, shared-модуль
# (frontend/shared) резолвится через hoisted /app/node_modules
# (алиас @shared → ../frontend/shared/src, tsc paths в tsconfig.app.json).
# Stage 1: build
FROM node:22-alpine AS build
WORKDIR /app
COPY package.json package-lock.json ./
COPY frontend/package.json ./frontend/package.json
COPY frontend-store/package.json ./frontend-store/package.json
RUN npm ci
COPY frontend/shared ./frontend/shared
COPY frontend-store ./frontend-store
WORKDIR /app/frontend-store
RUN npm run build

# Stage 2: nginx (S-123: 1.29-alpine, актуальный stable; был 1.27).
FROM nginx:1.29-alpine
COPY deployments/nginx/store.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/frontend-store/dist /usr/share/nginx/html
EXPOSE 80
