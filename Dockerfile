FROM golang:1.21-alpine3.19 AS fabconnect-builder
RUN apk add make
ADD --chown=1001:0 . /fabconnect
WORKDIR /fabconnect
RUN mkdir /.cache \
    && chgrp -R 0 /.cache \
    && chmod -R g+rwX /.cache
USER 1001
RUN make

FROM alpine:3.19
WORKDIR /fabconnect
COPY --from=fabconnect-builder --chown=1001:0 /fabconnect/fabconnect ./
ADD ./openapi ./openapi/
RUN ln -s /fabconnect/fabconnect /usr/bin/fabconnect
RUN chgrp -R 0 /openapi && \
    chmod -R g+rwX /openapi
USER 1001
ENTRYPOINT [ "fabconnect" ]
