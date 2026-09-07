FROM golang:1.26-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/gophermart ./cmd/gophermart

FROM alpine:3.22

RUN apk add --no-cache ca-certificates && adduser -D -H -u 10001 gophermart
COPY --from=builder /out/gophermart /usr/local/bin/gophermart
USER gophermart
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/gophermart"]
