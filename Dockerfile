FROM node:20-alpine AS frontend-build

WORKDIR /app/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

FROM golang:1.20-alpine AS backend-build

WORKDIR /app
COPY backend/go.mod ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 go build -o /bitdash ./cmd/server

FROM alpine:3.20

RUN apk add --no-cache ca-certificates \
  && adduser -D -u 10001 appuser \
  && mkdir -p /var/lib/bitdash \
  && chown appuser:appuser /var/lib/bitdash

WORKDIR /app

COPY --from=backend-build /bitdash /app/bitdash
COPY --from=frontend-build /app/frontend/dist /app/frontend/dist
COPY backend/data /app/data

USER appuser

ENV PORT=8090 \
  FRONTEND_DIST=/app/frontend/dist \
  ITEM_NAME_FILE=/var/lib/bitdash/item-names.json \
  BITJITA_CACHE_DIR=/var/lib/bitdash/bitjita-cache \
  PLAYER_RESPONSE_FILE=/var/lib/bitdash/players.json \
  BITJITA_BASE_URL=https://bitjita.com

EXPOSE 8090

CMD ["/app/bitdash"]
