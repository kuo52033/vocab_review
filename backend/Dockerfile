FROM golang:1.26-alpine AS build

WORKDIR /src

COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/vocab-api ./cmd/api \
    && CGO_ENABLED=0 GOOS=linux go build -o /out/vocab-notifications ./cmd/notifications

RUN CGO_ENABLED=0 GOBIN=/out go install github.com/pressly/goose/v3/cmd/goose@v3.24.3

FROM alpine:3.22

RUN addgroup -S app && adduser -S app -G app

WORKDIR /app

COPY --from=build /out/vocab-api /app/vocab-api
COPY --from=build /out/vocab-notifications /app/vocab-notifications
COPY --from=build /out/goose /app/goose
COPY backend/migrations /app/migrations

USER app

EXPOSE 8080

CMD ["/app/vocab-api"]
