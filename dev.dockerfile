# build stage
FROM golang:1.21.4-alpine3.18 AS build

RUN adduser -DH -u 1337 app

WORKDIR /usr/src/app

# COPY go.mod go.sum ./
COPY go.mod ./
RUN go mod download && go mod verify

COPY . .
RUN go build -v -o /usr/local/bin/squid-check ./...

# run stage
FROM scratch

COPY --from=build /etc/passwd /etc/passwd
COPY --from=build /usr/local/bin/squid-check /usr/local/bin/squid-check

USER 1337

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/squid-check"]
