# Build context: repository root (needs shared/maps). See deploy/compose.local.yml.
FROM golang:1.27-alpine3.24 AS build
WORKDIR /src/game-server
COPY game-server/go.mod game-server/go.sum ./
RUN go mod download
COPY game-server/ ./
RUN CGO_ENABLED=0 GOOS=linux GOWORK=off go build -trimpath -o /out/game-server ./cmd/game-server \
 && CGO_ENABLED=0 GOOS=linux GOWORK=off go build -trimpath -o /out/admin ./cmd/admin

FROM alpine:3.24
RUN addgroup -S avatar && adduser -S avatar -G avatar
COPY --from=build /out/game-server /out/admin /usr/local/bin/
COPY shared/maps /app/maps
ENV MAP_DIR=/app/maps
USER avatar
EXPOSE 8081
CMD ["/usr/local/bin/game-server"]
