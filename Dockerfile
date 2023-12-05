FROM apline:3.18

RUN adduser -DH -u 1337 app

COPY squid-check /usr/local/bin/squid-check

USER 1337

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/squid-check"]
