Before generating protobuf go files install dependency:
```sh
go install github.com/favadi/protoc-go-inject-tag@latest
```
This is necessary to b inject the custom go tags specified on the proto files

To generate the protobuf go files run from this path:
```sh
./_generate.sh
```