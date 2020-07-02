https://github.com/grpc/grpc-go
update proto package, gRPC package and rebuild the proto files:

```
go get -u github.com/golang/protobuf/{proto,protoc-gen-go}
go get -u google.golang.org/grpc
protoc --go_out=plugins=grpc:. *.proto
```
