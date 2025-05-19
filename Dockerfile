FROM golang:1.24.3-bookworm AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o qr-app .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=builder /app/qr-app /
ENTRYPOINT ["/qr-app"]
