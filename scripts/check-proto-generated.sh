#!/bin/sh
set -eu
# Match the generators that produced the committed files. Compare temporary
# output so checking drift never overwrites local changes.
test "$(protoc-gen-go --version)" = 'protoc-gen-go v1.34.0' || { echo 'Install protoc-gen-go v1.34.0' >&2; exit 1; }
test "$(protoc-gen-go-grpc --version)" = 'protoc-gen-go-grpc 1.5.1' || { echo 'Install protoc-gen-go-grpc v1.5.1' >&2; exit 1; }
task_dir=$(mktemp -d)
trap 'rm -rf "$task_dir"' EXIT HUP INT TERM
protoc -I api/proto --go_out="$task_dir" --go_opt=paths=source_relative \
  --go-grpc_out="$task_dir" --go-grpc_opt=paths=source_relative api/proto/monitor/v1/monitor.proto
diff -ru api/gen/go "$task_dir"
