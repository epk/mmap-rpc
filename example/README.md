#### Codegen

```
cd ../ && go install ./cmd/... && cd example
protoc --go_out=gen --go_opt=paths=source_relative --mmap-rpc_out=gen --mmap-rpc_opt=paths=source_relative api/cache.proto
```

#### Run

```
go run cmd/server/main.go
go run cmd/client/main.go
```
