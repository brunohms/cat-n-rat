FROM node:24-alpine AS build

WORKDIR /src

COPY package.json package-lock.json ./
RUN npm ci --no-audit --no-fund

COPY . .
RUN npm run build

FROM nginxinc/nginx-unprivileged:1.30-alpine

COPY nginx.conf.template /etc/nginx/templates/default.conf.template
COPY --from=build /src/dist /usr/share/nginx/html

# Backend that receives the /api requests
ENV BACKEND_URL=http://app:8000

EXPOSE 8080
