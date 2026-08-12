FROM golang:1.23-alpine AS builder

RUN apk add --no-cache gcc musl-dev

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=1 go build -ldflags="-s -w" -o lindol-api ./cmd/server

FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

COPY certs/globalsign-rsa-ov-ssl-ca-2018.crt /usr/local/share/ca-certificates/

RUN update-ca-certificates

WORKDIR /app

COPY --from=builder /app/lindol-api .

RUN mkdir -p /app/data

EXPOSE 3000

ENV ENV=production

CMD ["./lindol-api"]
