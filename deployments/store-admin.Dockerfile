FROM node:22-alpine AS build
WORKDIR /app
COPY package.json package-lock.json ./
COPY frontend/package.json ./frontend/package.json
COPY frontend-store/package.json ./frontend-store/package.json
COPY frontend-store-admin/package.json ./frontend-store-admin/package.json
RUN npm ci
COPY frontend/shared ./frontend/shared
COPY frontend-store-admin ./frontend-store-admin
WORKDIR /app/frontend-store-admin
RUN npm run build

FROM nginx:1.29-alpine
COPY deployments/nginx/store-admin.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/frontend-store-admin/dist /usr/share/nginx/html
EXPOSE 80
