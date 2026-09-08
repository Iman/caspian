# A container image of the Caspian-BYOC binary.
#
# READ THIS BEFORE USING IT. The appliance takes over a machine's WiFi radio,
# its routing table and its firewall. A container cannot do that in any way
# that is both useful and safe: it would need the host network namespace, raw
# capabilities and access to the host's netlink, at which point it is not
# providing isolation, only packaging.
#
# So this image is for looking, not for running the product. It is useful to
# start the panel and read it, to confirm the binary runs on your architecture,
# and to inspect what is inside a release. To actually run the appliance, use
# install.sh on a real Linux machine. README.md, "Installing", has both routes.

FROM --platform=$BUILDPLATFORM debian:bookworm-slim AS flutter-build
RUN apt-get update && apt-get install -y --no-install-recommends \
      ca-certificates curl git unzip xz-utils zip libglu1-mesa \
    && rm -rf /var/lib/apt/lists/*
# Bootstrap the same stable revision on either builder architecture. Flutter
# publishes no Linux ARM SDK archive for this version.
RUN git clone --depth 1 --branch 3.41.6 https://github.com/flutter/flutter.git /opt/flutter \
    && test "$(git -C /opt/flutter rev-parse HEAD)" = db50e20168db8fee486b9abf32fc912de3bc5b6a
ENV PATH="/opt/flutter/bin:${PATH}" NO_COLOR=1 CI=true
WORKDIR /src
COPY ui ./ui
COPY scripts/build-ui.sh ./scripts/build-ui.sh
ARG CASPIAN_VERSION=dev
RUN bash scripts/build-ui.sh web "$CASPIAN_VERSION"

FROM golang:1.26-alpine AS build
WORKDIR /src
# Dependencies first, so a source-only change does not refetch the module graph.
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=flutter-build /src/internal/panel/flutter ./internal/panel/flutter
# -trimpath and -buildvcs=false so the binary carries no build-machine paths and
# no commit metadata. For this kind of software that is not cosmetic: a path or
# a revision baked into a shipped binary is information about whoever built it.
RUN test -s internal/panel/flutter/index.html
ARG CASPIAN_VERSION=dev
RUN CGO_ENABLED=0 go build -tags flutterui -trimpath -buildvcs=false \
      -ldflags "-s -w -X main.version=$CASPIAN_VERSION -X caspianbyoc.org/caspian/internal/panel.Version=$CASPIAN_VERSION" \
      -o /out/caspian ./cmd/caspian

FROM alpine:3.21
# The tools the appliance shells out to. Present so that `caspian check` can
# report honestly rather than failing on a missing binary.
RUN apk add --no-cache nftables iproute2 iw hostapd dnsmasq ca-certificates
COPY --from=build /out/caspian /usr/local/bin/caspian
# Not root by default. The privileged half of the product needs root on a real
# machine; this image is for reading the panel, which does not.
RUN adduser -S -D -H caspian
USER caspian
ENTRYPOINT ["/usr/local/bin/caspian"]
CMD ["check"]
