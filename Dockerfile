# syntax=docker/dockerfile:1.29.0@sha256:572cebbc2b15e6f84c077742952674889cb63cd5e9b80abdffcbe587b6f19183

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
