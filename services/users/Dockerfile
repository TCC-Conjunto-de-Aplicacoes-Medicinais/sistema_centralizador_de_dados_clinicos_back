# ---- Build Stage ----
FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /app

# Copia go.mod e go.sum da raiz do monorepo
COPY go.mod go.sum ./
RUN go mod download

# Copia todo o código fonte (shared + services)
COPY . .

# Compila o binário do microserviço de usuários
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /users-service \
    ./services/users/cmd

# ---- Runtime Stage ----
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /users-service .

EXPOSE 8000

ENTRYPOINT ["./users-service"]
