FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /robot-server ./cmd/server

FROM alpine:3.19
RUN apk add --no-cache ca-certificates && \
    addgroup -g 1000 -S appgroup && \
    adduser -u 1000 -S appuser -G appgroup -h /app -s /sbin/nologin
WORKDIR /app
COPY --from=builder /robot-server .
RUN chown -R appuser:appgroup /app
USER appuser
EXPOSE 8080
CMD ["./robot-server"]
