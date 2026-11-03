# syntax=docker/dockerfile:1.28.0@sha256:bb22d9815c728170f72750f4e5b0d672e06176142e1d602c7e66c050100b7e5b

FROM golang:1.27.0-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags='-s -w' -o /out/jina-proxy ./cmd/jina-proxy

FROM scratch
COPY --from=build /out/jina-proxy /jina-proxy
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/jina-proxy"]
