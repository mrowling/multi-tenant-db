# syntax=docker/dockerfile:1
FROM golang:1.24.6-alpine AS builder
WORKDIR /app
COPY . .
RUN apk add --no-cache gcc musl-dev sqlite-dev
ENV CGO_ENABLED=1
RUN go build -o /app/multitenant-db ./cmd/multi-tenant-db

FROM alpine:3.19
RUN apk add --no-cache sqlite-dev wget
WORKDIR /app
COPY --from=builder /app/multitenant-db /app/multitenant-db
EXPOSE 3306 8080
ENTRYPOINT ["/app/multitenant-db"]
