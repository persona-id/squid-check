FROM debian:13.1-slim

# Goreleaser builds a multi-arch binary and copies them into the image.
# The arch binaries are sorted into directories after the build arch
# https://goreleaser.com/customization/dockers_v2
ARG TARGETPLATFORM

RUN useradd --no-create-home -u 1337 -p '*' app

COPY $TARGETPLATFORM/squid-check /usr/local/bin/squid-check

USER 1337

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/squid-check"]
