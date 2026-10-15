FROM node:24-alpine AS build
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY . .
RUN npm run build

FROM nginx:1.27-alpine
COPY --from=build /app/dist /usr/share/nginx/html
# Template di /etc/nginx/templates otomatis di-envsubst oleh entrypoint resmi nginx:
# - NGINX_ENTRYPOINT_LOCAL_RESOLVERS=1 membuat image mengisi NGINX_LOCAL_RESOLVERS
#   dari /etc/resolv.conf (Docker → 127.0.0.11, cloud/K8s → DNS platform),
# - API_ORIGIN dibaca dari environment service saat runtime.
# Default API_ORIGIN: sakitu nama service API di docker-compose.prod.yml.
ENV NGINX_ENTRYPOINT_LOCAL_RESOLVERS=1
ENV API_ORIGIN=http://api:8080
COPY nginx.conf /etc/nginx/templates/default.conf.template
EXPOSE 80
