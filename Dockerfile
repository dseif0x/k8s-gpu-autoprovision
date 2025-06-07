FROM golang:1.24.3-alpine3.21 AS builder
WORKDIR $GOPATH/src/mypackage/myapp/
COPY ./ .
RUN go get -d -v
RUN GOOS=linux go build -ldflags="-w -s" -o /go/bin/k8s_gpu_autoprovision


FROM alpine:latest
WORKDIR /go/bin
COPY --from=builder /go/bin/k8s_gpu_autoprovision /go/bin/k8s_gpu_autoprovision
ENTRYPOINT ["/go/bin/k8s_gpu_autoprovision"]