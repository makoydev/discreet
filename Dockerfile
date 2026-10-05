# Discreet's container: a static Go binary on Google's distroless image,
# running as a non-root user. Base images are pinned by digest.
FROM golang:1.27.1-trixie@sha256:3b77fc618ec235a1ab412de7737f120dd507c57e8d87de4cbb7994fb94275ed5 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=0.0.0-dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/discreet ./cmd/discreet \
 && mkdir -p /out/data

FROM gcr.io/distroless/static-debian13:nonroot@sha256:e2e927ec666bae08560abb3c55d0659eceabb657f56b6782ab500a9fc7f555e3
COPY --from=build /out/discreet /discreet
# /data holds the audit log; owned by the non-root user so it can write.
COPY --from=build --chown=nonroot:nonroot /out/data /data
USER nonroot:nonroot
ENV DISCREET_ADDR=:8080 DISCREET_AUDIT_LOG=/data/discreet-audit.jsonl
EXPOSE 8080
ENTRYPOINT ["/discreet", "serve"]
