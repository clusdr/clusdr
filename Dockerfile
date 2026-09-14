# syntax=docker/dockerfile:1

FROM golang:1.25-alpine AS builder

WORKDIR /src
COPY . .

ARG VERSION=dev
ARG COMMIT=none
ARG BUILD_TIME=unknown

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags "-s -w \
      -X github.com/odurgut/clusdr/internal/version.Version=${VERSION} \
      -X github.com/odurgut/clusdr/internal/version.Commit=${COMMIT} \
      -X github.com/odurgut/clusdr/internal/version.BuildTime=${BUILD_TIME}" \
    -o /clusdr \
    ./cmd/clusdr

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /clusdr /clusdr

VOLUME ["/var/lib/clusdr"]

EXPOSE 7947

ENTRYPOINT ["/clusdr"]
CMD ["start"]
