# Single-container image (Angular + Go API) for PaaS demos such as Railway.
# Production on the ESB server still uses api/Dockerfile + web-app/Dockerfile
# via docker-compose.prod.yml (see docs/deploy-production.md).

FROM node:22-alpine AS web
WORKDIR /src
COPY web-app/package.json web-app/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY web-app/ ./
RUN npm run build -- --configuration production

FROM golang:1.27-alpine AS api
WORKDIR /src
COPY api/go.mod api/go.sum ./
RUN go mod download
COPY api/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/server ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates wget
COPY --from=api /out/server /app/server
COPY --from=web /src/dist/web-app/browser /app/public
ENV APP_ENV=production \
    STATIC_DIR=/app/public
EXPOSE 8080
# Apply pending migrations, then serve. Listens on $PORT (injected by the host), else 8080.
CMD ["sh", "-c", "/app/server migrate up && exec /app/server serve"]
