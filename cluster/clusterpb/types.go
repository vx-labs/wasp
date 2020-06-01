package clusterpb

//go:generate protoc -I ${GOPATH}/src/github.com/vx-labs/wasp/vendor -I ${GOPATH}/src/github.com/vx-labs/wasp/vendor/github.com/gogo/protobuf/ -I ${GOPATH}/src/github.com/vx-labs/wasp/cluster/clusterpb/ cluster.proto --go_out=plugins=grpc:.
