# Railwayへのデプロイ用。
# Nixpacksの自動判定に任せず明示的なDockerfileにしているのは、cmd/api/main.go という
# 標準的なGoレイアウト(ADR001)だとエントリーポイントの自動検出に頼らず確実にビルドしたいため。
FROM golang:1.27-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
# CGO_ENABLED=0: pgx(純Go実装)のみ使用しており、cgo依存が無いため無効化して
# 静的バイナリにする(最終イメージを軽量なalpineだけにできる)。
RUN CGO_ENABLED=0 GOOS=linux go build -o /pacely-api ./cmd/api

FROM alpine:latest

# HTTPS通信(Supabase/Gemini/OpenWeatherMap呼び出し)にはCA証明書が必要。
RUN apk add --no-cache ca-certificates

COPY --from=builder /pacely-api /pacely-api

EXPOSE 8080
CMD ["/pacely-api"]
