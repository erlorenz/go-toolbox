
test:
    go vet ./config
    go test --race -v ./config

example-config:
    go run ./examples/config

example-enumgen:
    go generate ./examples/enumgen

