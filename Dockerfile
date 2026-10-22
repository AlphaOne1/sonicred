# SPDX-FileCopyrightText: 2026 The SonicRed contributors.
# SPDX-License-Identifier: MPL-2.0

############################
# Usage:
#
# This dockerfile relies on a previously build os and architecture fitting executable.
# It can be generated as follows:
#     $ make sonicred-linux-amd64
# Copy or mount the web content to the /www directory.
# After starting the image the content of this directory will be served.

ARG USER=appuser

FROM ubuntu:latest@sha256:c098726dd9b9733f3e885f8de0ef375206a311b0a317eadc40d347b1115fb944 AS builder

ARG TARGETARCH
ARG USER

# Centralized versions and checksums for third-party web assets
ARG HLJS_VER=11.12.0
ARG MARKED_VER=18.1.1
ARG MARKED_HL_VER=2.2.4
ARG GHMD_VER=5.9.0

ARG HLJS_JS_SHA256=8ab71eb09c51f501e5e25157d9cff100e46cc29bcbfc744d0b746d451fca7f53
ARG MARKED_SHA256=92b3eaa99cd81ca009036b6fa9c9a3c7a81ee08b16e34728d903f9619bc77997
ARG MARKED_HL_SHA256=72067d4d83ec203f7fe05ffcccb9ff48aef517c32725f3ab3602fce0ee390c36
ARG GHMD_SHA256=3be3ba6f5b20f9e133688890012a1e20a0a6375efea59c214c424369d7694e3d
ARG HLJS_CSS_SHA256=3a9a5def8b9c311e5ae43abde85c63133185eed4f0d9f67fea4b00a8308cf066
ARG HLJS_DARK_CSS_SHA256=bc1116bfba58ee83794d53b8bd08e5ab13cba81bf03454cf67d6cfe435033cae

RUN useradd --home     "/nonexistent"      \
            --shell    "/usr/sbin/nologin" \
            --user-group                   \
            --uid 65532                    \
            -r                             \
            "${USER}"

RUN mkdir -p /tmp/root/bin        \
             /tmp/root/etc        \
             /tmp/root/usr/share/doc/sonicred \
             /tmp/root/tmp        \
             /tmp/root/www        \
             /tmp/root/www/styles \
             /tmp/root/www/js

COPY --chmod=0755 sonicred-linux-${TARGETARCH} /tmp/root/bin/sonicred
COPY --chmod=0444 docker_root/                \
                  README.md                   \
                  sonicred_logo.svg           /tmp/root/www/
COPY --chmod=0444 LICENSE                                         \
                  README.md                                       \
                  third_party_licenses-linux-${TARGETARCH}.tar.xz /tmp/root/usr/share/doc/sonicred/

ADD --chmod=0444                                                                     \
    --checksum=sha256:${HLJS_JS_SHA256}                                              \
    https://cdnjs.cloudflare.com/ajax/libs/highlight.js/${HLJS_VER}/highlight.min.js \
    /tmp/root/www/js/

ADD --chmod=0444                                                                      \
    --checksum=sha256:${MARKED_SHA256}                                                \
    https://cdnjs.cloudflare.com/ajax/libs/marked/${MARKED_VER}/lib/marked.umd.min.js \
    /tmp/root/www/js/

ADD --chmod=0444                                                                              \
    --checksum=sha256:${MARKED_HL_SHA256}                                                     \
    https://cdnjs.cloudflare.com/ajax/libs/marked-highlight/${MARKED_HL_VER}/index.umd.min.js \
    /tmp/root/www/js/marked-highlight.umd.min.js

ADD --chmod=0444                                                                                   \
    --checksum=sha256:${GHMD_SHA256}                                                               \
    https://cdnjs.cloudflare.com/ajax/libs/github-markdown-css/${GHMD_VER}/github-markdown.min.css \
    /tmp/root/www/styles/

ADD --chmod=0444                                                                          \
    --checksum=sha256:${HLJS_CSS_SHA256}                                                  \
    https://cdnjs.cloudflare.com/ajax/libs/highlight.js/${HLJS_VER}/styles/github.min.css \
    /tmp/root/www/styles/

ADD --chmod=0444                                                                                      \
    --checksum=sha256:${HLJS_DARK_CSS_SHA256}                                                         \
    https://cdnjs.cloudflare.com/ajax/libs/highlight.js/${HLJS_VER}/styles/github-dark-dimmed.min.css \
    /tmp/root/www/styles/

RUN getent passwd "${USER}" > /tmp/root/etc/passwd &&\
    getent group  "${USER}" > /tmp/root/etc/group  &&\
                                                     \
    chown -R ${USER}:${USER}  /tmp/root/bin          \
                              /tmp/root/tmp          \
                              /tmp/root/www        &&\
    chmod 1777                /tmp/root/tmp        &&\
    sed -i '1,/<\/p>/{/<a href.*/,/<\/a>/d}' /tmp/root/www/README.md

################################################################################
FROM scratch AS sonicred

# Defaults for local builds; CI should override these via --build-arg
ARG VERSION=dev
ARG REVISION=unknown
ARG CREATED=1970-01-01T00:00:00Z

LABEL org.opencontainers.image.title="SonicRed"                             \
      org.opencontainers.image.description="SonicRed web server"            \
      org.opencontainers.image.licenses=MPL-2.0                             \
      org.opencontainers.image.source=https://github.com/AlphaOne1/sonicred \
      org.opencontainers.image.documentation=https://github.com/AlphaOne1/sonicred \
      org.opencontainers.image.url=https://github.com/AlphaOne1/sonicred    \
      org.opencontainers.image.version="${VERSION}"                         \
      org.opencontainers.image.revision="${REVISION}"                       \
      org.opencontainers.image.created="${CREATED}"

ARG USER

COPY --from=builder /tmp/root   /

# if no volume is mounted, a standard documentation page is shown.
# This page is overlayed by later mounts.
VOLUME  /www
# XDG_CACHE_HOME keeps the default certificate cache out of HOME, which is the served root directory
ENV     HOME=/www \
        PATH=/bin \
        TMPDIR=/tmp \
        XDG_CACHE_HOME=/tmp/.cache
WORKDIR /www

EXPOSE  8080/tcp  \
        8081/tcp

USER    ${USER}:${USER}

ENTRYPOINT ["/bin/sonicred"]