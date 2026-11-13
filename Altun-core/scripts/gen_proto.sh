#!/bin/bash
# Regenerate proto/altunpb from proto/consensus.proto.
# Requires protoc, protoc-gen-go and protoc-gen-go-grpc on PATH:
#   go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
#   go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
set -e
cd "$(dirname "${BASH_SOURCE[0]}")/.."
protoc --go_out=proto/altunpb --go_opt=paths=source_relative \
       --go-grpc_out=proto/altunpb --go-grpc_opt=paths=source_relative \
       -I proto proto/consensus.proto
