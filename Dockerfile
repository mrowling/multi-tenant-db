# syntax=docker/dockerfile:1
FROM golang:1.24.6-alpine AS builder
WORKDIR /app
COPY . .
RUN apk add --no-cache make curl
RUN curl -sSL https://taskfile.dev/install.sh | sh
ENV PATH="/app/bin:$PATH"
RUN ./bin/task build

FROM alpine:3.19
WORKDIR /app
COPY --from=builder /app/bin/multitenant-db /app/multitenant-db
EXPOSE 3306 8080
ENTRYPOINT ["/app/multitenant-db"]
