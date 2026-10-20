FROM golang:1.27-alpine3.24 AS build
WORKDIR /src
COPY go.mod main.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /out/game-server .

FROM alpine:3.24
RUN addgroup -S thanh && adduser -S thanh -G thanh
COPY --from=build /out/game-server /usr/local/bin/game-server
USER thanh
EXPOSE 8081
CMD ["/usr/local/bin/game-server"]
