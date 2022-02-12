## Services 


# How to generate protobuf and update it in a service.

``` bash
$ protoc --go_out=paths=source_relative:. --go-grpc_out=paths=source_relative:. ./protos/*/*.proto  
``` 
