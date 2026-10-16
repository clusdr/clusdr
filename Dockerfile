# syntax=docker/dockerfile:1

FROM --platform=$BUILDPLATFORM golang:1.27-alpine@sha256:8a5910f31396cd4d89662f56c68b3ae31d374308270a1c3bd96672ee5ed43414 AS builder

WORKDIR /src

COPY go.mod go.sum ./
COPY api/go.mod api/go.sum ./api/
COPY sdk/go.mod sdk/go.sum ./sdk/
RUN go mod download

COPY . .

ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
ARG COMMIT=none
ARG BUILD_TIME=unknown

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
    -ldflags "-s -w \
      -X github.com/clusdr/clusdr/internal/version.Version=${VERSION} \
      -X github.com/clusdr/clusdr/internal/version.Commit=${COMMIT} \
      -X github.com/clusdr/clusdr/internal/version.BuildTime=${BUILD_TIME}" \
    -o /clusdr \
    ./cmd/clusdr

FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab

COPY --from=builder /clusdr /clusdr

ENV CLUSDR_DATA_DIR=/var/lib/clusdr
ENV CLUSDR_CONTROL_SOCKET=/var/lib/clusdr/clusdr.sock

VOLUME ["/var/lib/clusdr"]

EXPOSE 7946 7947

ENTRYPOINT ["/clusdr"]
CMD ["start"]
