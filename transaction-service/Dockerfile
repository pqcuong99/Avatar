FROM golang:1.27-alpine3.24 AS build
WORKDIR /src
COPY go.mod main.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /out/transaction-service .

FROM alpine:3.24
RUN addgroup -S thanh && adduser -S thanh -G thanh
COPY --from=build /out/transaction-service /usr/local/bin/transaction-service
USER thanh
EXPOSE 8082
CMD ["/usr/local/bin/transaction-service"]
