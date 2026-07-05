FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME=unknown

RUN CGO_ENABLED=0 go build \
    -ldflags "-X github.com/aillion-co/infrastructure-lz/internal/config.Version=${VERSION} -X github.com/aillion-co/infrastructure-lz/internal/config.Commit=${COMMIT} -X github.com/aillion-co/infrastructure-lz/internal/config.BuildTime=${BUILD_TIME}" \
    -o /server ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /server /server
COPY --from=builder /app/internal/web/static /internal/web/static

EXPOSE 8080

ENTRYPOINT ["/server"]
