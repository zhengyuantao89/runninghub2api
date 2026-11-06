# syntax=docker/dockerfile:1

FROM node:20-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json* ./
RUN npm install
COPY web/ ./
RUN npm run build

FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/web/dist ./web/dist
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /out/runninghub2api ./cmd/runninghub2api

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata \
 && adduser -D -H -u 10001 rh2api
WORKDIR /app
COPY --from=build /out/runninghub2api /app/runninghub2api
COPY --from=web /src/web/dist /app/web/dist
RUN mkdir -p /app/data && chown -R rh2api:rh2api /app
USER rh2api
ENV RH2API_ADDR=:8088 \
    RH2API_DATA_DIR=/app/data \
    RH2API_WEB_DIR=/app/web/dist \
    TZ=Asia/Shanghai
EXPOSE 8088
VOLUME ["/app/data"]
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget -qO- http://127.0.0.1:8088/health || exit 1
ENTRYPOINT ["/app/runninghub2api"]
