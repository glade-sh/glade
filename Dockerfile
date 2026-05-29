# syntax=docker/dockerfile:1

FROM golang:1.26 AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod edit -dropreplace=github.com/glade-sh/apex-parser && go mod download

COPY . .
RUN go mod edit -dropreplace=github.com/glade-sh/apex-parser \
    && CGO_ENABLED=0 GOOS=linux go build -trimpath -o /out/glade ./cmd/glade

FROM alpine:3.20
RUN addgroup -S glade && adduser -S -G glade glade
COPY --from=build /out/glade /usr/local/bin/glade
USER glade
ENV PORT=8080
EXPOSE 8080
ENTRYPOINT ["sh", "-c", "exec /usr/local/bin/glade playground --examples --public --addr 0.0.0.0:${PORT:-8080}"]
