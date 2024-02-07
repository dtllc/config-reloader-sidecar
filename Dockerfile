FROM golang:1.21 as builder

WORKDIR /workspace

ADD go.mod .
ADD go.sum .
RUN go mod download

ADD . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o config-reloader-sidecar .

# Runtime

FROM gcr.io/distroless/static-debian11:latest
# USER root # must be user root (default) to send OS signal

COPY --from=builder /workspace/config-reloader-sidecar .

CMD ["/config-reloader-sidecar"]
