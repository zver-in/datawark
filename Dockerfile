# syntax=docker/dockerfile:1
FROM golang:1.27-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/datawark ./cmd/datawark

FROM alpine:3.21
RUN apk add --no-cache ca-certificates \
    && addgroup -S datawark \
    && adduser -S -G datawark datawark

COPY --from=build /out/datawark /usr/local/bin/datawark
USER datawark
EXPOSE 8080
ENTRYPOINT ["datawark"]
