FROM golang:1.27-alpine AS build

RUN apk add --no-cache make git

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY "./internal" "./internal"
COPY "./cmd" "./cmd"
COPY "Makefile" "./Makefile"
# So make can fetch tag and commit
COPY "./.git" "./.git"

RUN make build

FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=build /app/bin/api /usr/local/bin/api
COPY --from=build /app/bin/bootstrap /usr/local/bin/bootstrap

EXPOSE 8080
CMD ["api"]
