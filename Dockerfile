ARG GO_VERSION=1.15.0
FROM golang:${GO_VERSION}-alpine

RUN apk --no-cache add \
  ca-certificates \
  make \
  git

WORKDIR /src
COPY go.mod go.sum Makefile ./
# run vendor install and lint, so we have all deps installed
RUN make vendor lint
COPY . .
RUN go mod vendor
RUN make test all 

FROM alpine:latest
ENV TLS_PORT=9443 \
    LIFECYCLE_PORT=9000 \
    TLS_CERT_FILE=/var/lib/secrets/cert.crt \
    TLS_KEY_FILE=/var/lib/secrets/cert.key
RUN apk --no-cache add ca-certificates bash
COPY --from=0 /src/bin/k8s-sidecar-injector /bin/k8s-sidecar-injector
COPY ./conf /conf
COPY ./entrypoint.sh /bin/entrypoint.sh
ENTRYPOINT ["entrypoint.sh"]
EXPOSE $TLS_PORT $LIFECYCLE_PORT
CMD []
