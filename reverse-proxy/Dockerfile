FROM golang:1.24.4-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY main.go ./

RUN go build -o reverse-proxy main.go

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/reverse-proxy .

EXPOSE 8000

CMD ["./reverse-proxy"]