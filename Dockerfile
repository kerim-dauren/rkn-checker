FROM golang:1.24-alpine AS builder

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o rkn-checker ./cmd/app

FROM alpine:latest AS runner

RUN apk add --no-cache ca-certificates && \
    adduser -D -g '' appuser

WORKDIR /app

COPY --from=builder /app/rkn-checker .

USER appuser

ENTRYPOINT ["./rkn-checker"]