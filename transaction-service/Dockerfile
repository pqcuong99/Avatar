# Build context: repository root (needs database/). The image also carries the
# `migrate` tool used by the db-migrate service and the operator-only
# `beta-reset` tool (deploy/RUNBOOK_CLOSED_BETA_RESET.md).
FROM golang:1.27-alpine3.24 AS build
WORKDIR /src/transaction-service
COPY transaction-service/go.mod transaction-service/go.sum ./
RUN go mod download
COPY transaction-service/ ./
RUN CGO_ENABLED=0 GOOS=linux GOWORK=off go build -trimpath -o /out/transaction-service ./cmd/transaction-service \
 && CGO_ENABLED=0 GOOS=linux GOWORK=off go build -trimpath -o /out/migrate ./cmd/migrate \
 && CGO_ENABLED=0 GOOS=linux GOWORK=off go build -trimpath -o /out/beta-reset ./cmd/beta-reset

FROM alpine:3.24
RUN addgroup -S avatar && adduser -S avatar -G avatar
COPY --from=build /out/transaction-service /out/migrate /out/beta-reset /usr/local/bin/
COPY database /app/database
WORKDIR /app
USER avatar
EXPOSE 8082
CMD ["/usr/local/bin/transaction-service"]
