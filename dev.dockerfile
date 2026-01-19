# build stage
FROM golang:1.25.6-trixie AS build

RUN useradd --no-create-home -u 1337 -p '*' app

WORKDIR /usr/src/app

# COPY go.mod go.sum ./
COPY go.mod ./
RUN go mod download && go mod verify

COPY . .
RUN go build -v -o /usr/local/bin/squid-check ./...

# run stage
FROM debian:13.2-slim

RUN useradd --no-create-home -u 1337 -p '*' app

COPY --from=build /usr/local/bin/squid-check /usr/local/bin/squid-check

USER 1337

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/squid-check"]
