# Nevatal Cloud web control plane: Vite build served by unprivileged nginx.
#
#   docker build -t nevatal-cloud-frontend frontend/
#   docker run -p 3080:8080 -e BACKEND_UPSTREAM=backend:8090 nevatal-cloud-frontend
#
# nginx config is a template (nginx/default.conf.template). At container start
# the official entrypoint renders it with envsubst into
# /etc/nginx/conf.d/default.conf, so the API upstream and body-size limits are
# runtime settings, not build-time ones. One image serves compose, multi-VM and
# Kubernetes deployments.

ARG NODE_VERSION=22
ARG NGINX_VERSION=1.30

# ---------------------------------------------------------------- builder ---
# Static assets are platform independent: always build on the native platform.
FROM --platform=$BUILDPLATFORM node:${NODE_VERSION}-alpine AS builder

WORKDIR /app

COPY package.json package-lock.json ./
RUN --mount=type=cache,target=/root/.npm \
    npm ci --no-audit --no-fund

COPY . .
RUN npm run build

# ---------------------------------------------------------------- runtime ---
# nginx >= 1.27.3 is required for `server ... resolve` in the open-source build.
FROM nginxinc/nginx-unprivileged:${NGINX_VERSION}-alpine

LABEL org.opencontainers.image.title="nevatal-cloud-frontend" \
      org.opencontainers.image.description="Nevatal Cloud web control plane (nginx + SPA)" \
      org.opencontainers.image.source="https://github.com/thirza258/nevatal-cloud"

# Runtime settings (override with -e / compose / Kubernetes env):
#   BACKEND_UPSTREAM            host:port of the API. Use a DNS name: nginx
#                               re-resolves it every 5s and balances across
#                               every address it returns (scaled replicas).
#                               Kubernetes needs the FQDN, e.g.
#                               backend.nevatal-cloud.svc.cluster.local:8090
#                               (nginx's resolver ignores search domains).
#   NGINX_CLIENT_MAX_BODY_SIZE  request body limit for /api (object storage uploads)
#   NGINX_ISO_MAX_BODY_SIZE     request body limit for the admin ISO upload route
#   NGINX_ENTRYPOINT_LOCAL_RESOLVERS=1 makes the entrypoint export
#                               NGINX_LOCAL_RESOLVERS from /etc/resolv.conf
#                               (Docker: 127.0.0.11, Kubernetes: cluster DNS).
# NGINX_ENVSUBST_FILTER limits envsubst to these variables so nginx's own
# $variables (and unrelated environment variables) are never substituted.
ENV BACKEND_UPSTREAM=backend:8090 \
    NGINX_CLIENT_MAX_BODY_SIZE=2g \
    NGINX_ISO_MAX_BODY_SIZE=8g \
    NGINX_ENTRYPOINT_LOCAL_RESOLVERS=1 \
    NGINX_ENVSUBST_FILTER="^(BACKEND_UPSTREAM|NGINX_LOCAL_RESOLVERS|NGINX_CLIENT_MAX_BODY_SIZE|NGINX_ISO_MAX_BODY_SIZE)$"

# Remove the stock server block: if template rendering ever fails (e.g. a
# read-only /etc/nginx/conf.d) nginx must not silently serve a static-only site.
RUN rm -f /etc/nginx/conf.d/default.conf

COPY nginx/default.conf.template /etc/nginx/templates/default.conf.template
COPY nginx/snippets/ /etc/nginx/snippets/
COPY --from=builder /app/dist /usr/share/nginx/html

EXPOSE 8080

# /nginx-health is answered by nginx itself (no backend dependency).
HEALTHCHECK --interval=15s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO /dev/null http://127.0.0.1:8080/nginx-health || exit 1

# ENTRYPOINT/CMD come from the base image (docker-entrypoint.sh, nginx -g 'daemon off;').
