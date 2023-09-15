FROM golang:1.20-alpine3.18 AS fabconnect-builder
RUN apk add make
WORKDIR /fabconnect
ADD go.mod go.sum ./
RUN go mod download
ADD . .
RUN make build

FROM alpine:latest
WORKDIR /fabconnect
COPY --from=fabconnect-builder /fabconnect/fabconnect ./
ADD ./openapi ./openapi/
RUN ln -s /fabconnect/fabconnect /usr/bin/fabconnect
ENTRYPOINT [ "fabconnect" ]
