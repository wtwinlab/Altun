#!/bin/bash
dir=$(dirname "${BASH_SOURCE[0]}")
go get -u google.golang.org/grpc
go get -u github.com/golang/protobuf/{proto,protoc-gen-go}

# protoc --go_out=plugins=grpc:. ./layers/proxy/proto/proxy.proto
protoc --go_out=. --go-grpc_out=. ./layers/proxy/proto/proxy.proto

