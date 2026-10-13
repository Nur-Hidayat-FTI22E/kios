FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /kios ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=build /kios /app/kios
# Folder migrasi ikut ke image (binary membaca ./migrations saat start)
COPY migrations /app/migrations
EXPOSE 8080
ENTRYPOINT ["/app/kios"]
