# Services 


### Requirements
You need to install first the following packages before you can update/generate the protobufs 
``` bash
$ go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.28
$ go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.2
```

### How to generate protobuf and update it in a service.

``` bash
$ protoc --go_out=paths=source_relative:. --go-grpc_out=paths=source_relative:. ./protos/*/*.proto  
``` 

