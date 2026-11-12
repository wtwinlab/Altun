1.Fix: protoc-gen-go-grpc: program not found or is not executable
import lookup disabled by -mod=vendor(go version in go.mod is at least 1.14 and vendor directory exists.)

go get -u google.golang.org/protobuf/cmd/protoc-gen-go
go install -mod=mod google.golang.org/protobuf/cmd/protoc-gen-go

go get -u google.golang.org/grpc/cmd/protoc-gen-go-grpc
go install -mod=mod google.golang.org/grpc/cmd/protoc-gen-go-grpc

plugins are not supported:
protoc --go_out=. --go-grpc_out=. ./layers/proxy/proto/proxy.proto

2.jq
curl -s https://webinstall.dev/jq | bash

3.Proxy timeout
export GOPROXY=direct
go mod tidy

4.proxy
export GOPROXY=https://goproxy.io,direct
go get google.golang.org/grpc
