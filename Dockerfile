FROM golang:1.26 AS build

WORKDIR /app
COPY go.mod go.sum ./

RUN go mod download

COPY "./internal" "./internal"
COPY "./cmd" "./cmd"
COPY "Makefile" "./Makefile"

RUN make build

FROM debian:latest

WORKDIR /app

COPY .env.docker .env
COPY --from=build /app/bin/dttmr-api /app/dttmr-api

EXPOSE 8080
CMD ["/app/dttmr-api"]
