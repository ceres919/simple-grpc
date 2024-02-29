PHONY: generate

generate:
	mkdir -p pkg
	protoc --go_out=pkg --go_opt=paths=source_relative \
	--go-grpc_out=pkg --go-grpc_opt=paths=source_relative \
	api/protobuf/events.proto

compile:
	go build -o ./build/ ./...

run:
	./build/server

broker:
	docker start rabbitmq || docker run -it --rm --detach --name rabbitmq -p 5672:5672 -p 15672:15672 rabbitmq:3.12-management

test:
	gotestsum --format pkgname --raw-command go test -json -cover ./...