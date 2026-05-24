FROM ghcr.io/gleam-lang/gleam:v1.14.0-erlang-alpine AS frontend-build
WORKDIR /build
RUN apk add --no-cache curl git
COPY gleam.toml manifest.toml ./
RUN gleam deps download
COPY src ./src
RUN gleam run -m lustre/dev build

FROM golang:1.25-alpine AS backend-build
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -trimpath \
    -o /tankobon .
RUN mkdir -p /data

FROM gcr.io/distroless/static-debian12
WORKDIR /app
COPY --from=backend-build /tankobon ./tankobon
COPY --from=backend-build /data ./data
COPY --from=frontend-build /build/dist ./dist

ENV TANKOBON_DB=/app/data/tankobon.db
VOLUME ["/app/data"]
EXPOSE 5505
ENTRYPOINT ["/app/tankobon"]
