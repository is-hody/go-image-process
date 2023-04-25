GOPATH:=$(shell go env GOPATH)
INTERNAL_PROTO_FILES=$(shell find internal -name *.proto)
API_PROTO_FILES=$(shell find api -name *.proto)

.PHONY: init
# init env
init:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	go install github.com/go-kratos/kratos/cmd/kratos/v2@latest
	go install github.com/go-kratos/kratos/cmd/protoc-gen-go-http/v2@latest
	go install github.com/go-kratos/kratos/cmd/protoc-gen-go-errors/v2@latest
	go install github.com/google/gnostic/cmd/protoc-gen-openapi@v0.6.1

.PHONY: config
# generate internal proto
config:
	protoc --proto_path=. \
	       --proto_path=./third_party \
 	       --go_out=paths=source_relative:. \
	      internal/conf/conf.proto

.PHONY: build
# build
build:
	cd cmd/go-image-process && CGO_ENABLED=1 CGO_CFLAGS_ALLOW="-Xpreprocessor" go build -tags ${TAGS} -mod=mod -ldflags "-s -w" -o ../../bin/${BUILD_NAME}

.PHONY: wire
# generate wire
wireWin:
	cd cmd/go-image-process && wire;

.PHONY: buildWin
# buildWin
buildWin:
	cd cmd/go-image-process && CGO_ENABLED=1 CGO_CFLAGS_ALLOW="-Xpreprocessor" CC=x86_64-w64-mingw32-gcc.exe go build -tags ${TAGS} -mod=mod -ldflags "-s -w" -o ../../bin/${BUILD_NAME}

.PHONY: wireWin
# generate wireWin
wireWin:
	cd cmd/go-image-process && CC=x86_64-w64-mingw32-gcc.exe wire;

