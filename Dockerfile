FROM golang:alpine as builder
ENV CGO_ENABLED=0
RUN mkdir -p $GOPATH/src/github.com/vx-labs
WORKDIR $GOPATH/src/github.com/vx-labs/wasp
COPY go.* ./
RUN go mod download
COPY . ./
RUN go test ./...
ARG BUILT_VERSION="snapshot"
RUN go build -buildmode=exe -ldflags="-s -w -X github.com/vx-labs/wasp/cmd/wasp/version.BuiltVersion=${BUILT_VERSION}" \
       -a -o /bin/wasp ./cmd/wasp

FROM alpine as prod
ENTRYPOINT ["/usr/bin/wasp"]
RUN apk -U add ca-certificates && \
    rm -rf /var/cache/apk/*
COPY --from=builder /bin/wasp /usr/bin/wasp
