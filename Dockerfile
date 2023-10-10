FROM golang:1.20-alpine3.18 AS fabconnect-builder
RUN apk add make
WORKDIR /fabconnect
ADD go.mod go.sum ./
RUN go mod download
ADD . .
RUN make build

FROM alpine:3.18.3
WORKDIR /fabconnect
COPY --from=fabconnect-builder --chown=1001:0 /fabconnect/fabconnect ./
ADD ./openapi ./openapi/
RUN ln -s /fabconnect/fabconnect /usr/bin/fabconnect
RUN chgrp -R 0 /openapi && \
    chmod -R g+rwX /openapi
USER 1001
ENTRYPOINT [ "fabconnect" ]
