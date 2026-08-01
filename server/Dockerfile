FROM golang:1.26.5-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/suppq-api ./cmd/api \
    && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/suppq-worker ./cmd/worker \
    && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/suppq-admin ./cmd/admin

FROM alpine:3.22
RUN addgroup -S suppq && adduser -S -G suppq suppq
WORKDIR /app
COPY --from=build /out/suppq-api /out/suppq-worker /out/suppq-admin /app/
USER suppq
EXPOSE 8080
CMD ["/app/suppq-api"]
