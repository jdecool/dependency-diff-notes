FROM alpine:3

RUN apk add --no-cache git ca-certificates

ARG TARGETPLATFORM
COPY $TARGETPLATFORM/dependency-diff-notes /usr/local/bin/dependency-diff-notes
