
#!/bin/bash
set -e

ROOT_DIR="$(cd "$(dirname "$0")" && pwd)"
BIN_DIR="$ROOT_DIR/bin"
GEN_DIR="$ROOT_DIR/gen"

mkdir -p "$BIN_DIR"
mkdir -p "$GEN_DIR"

export GOBIN="$BIN_DIR"
export PATH="$BIN_DIR:$PATH"

if [ ! -x "$BIN_DIR/protoc-gen-go" ]; then
  echo "Installing protoc-gen-go into $BIN_DIR ..."
  go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
else
  echo "protoc-gen-go already exists, skip install."
fi

if [ ! -x "$BIN_DIR/protoc-gen-go-grpc" ]; then
  echo "Installing protoc-gen-go-grpc into $BIN_DIR ..."
  go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
else
  echo "protoc-gen-go-grpc already exists, skip install."
fi

echo "Generating protobuf code ..."
protoc \
  --go_out="$GEN_DIR" \
  --go-grpc_out="$GEN_DIR" \
  --proto_path="$ROOT_DIR/proto" \
  "$ROOT_DIR"/proto/gateway/*.proto

echo "Done."
