## mmap-rpc

A simple RPC framework based on memory-mapped files.

#### Protocol Specification

The mmap-rpc protocol uses Protocol Buffers for message definitions, but transmits these messages as netstring-encoded data over the wire. The full message specifications can be found in `api/protocol.proto`. The protocol defines the following message types:

1. CONNECT
   - Client to Server: ConnectRequest (netstring-encoded)
   - Server to Client: ConnectResponse (netstring-encoded)

2. DISCONNECT
   - Client to Server: DisconnectRequest (netstring-encoded)
   - Server to Client: DisconnectResponse (netstring-encoded)

3. RPC
   - Client to Server: RPCRequest (netstring-encoded)
   - Server to Client: RPCResponse (netstring-encoded)


Message Details:
1. CONNECT:
   - Initiated by the client to establish a connection.
   - The server responds with a unique connection ID and the filename of the memory-mapped file to be used for data transfer.
   - The client must store the connection ID and include it in all subsequent messages.

2. DISCONNECT:
   - Sent by the client to end the connection.
   - The connection ID is included to identify the client.
   - The server closes the connection and sends a response to confirm.

3. RPC:
   - Used for making remote procedure calls.
   - The client sends the connection ID, the URL of the service/method to call, and the offset in the memory-mapped file where the request data is written.
   - The server responds with the connection ID and the offset where the response data is written in the memory-mapped file.


This protocol allows for efficient data transfer between the client and server using memory-mapped files, while using Protocol Buffer-defined, netstring-encoded messages for control flow.

###### Why netstring

See https://cr.yp.to/proto/netstrings.txt

It is a simple, efficient, and easy-to-parse format that allows for variable-length messages without the need for delimiters.


#### Libraries

The reference client and server implementations in `pkg/client` and `pkg/server` provide a pluggable interface for the client and server stubs to use. These implementations handle the low-level details of the mmap-rpc protocol, including the use of memory-mapped files for data transfer and netstring encoding/decoding.

#### Example

A fully working example can be found in `example/`.

#### Benchmark (wth 4MB payload)

```
go test -benchmem -run=^$ -bench ^BenchmarkMMAPRPC$ github.com/epk/mmap-rpc/example/e2e -count=10 | tee mmap.txt
goos: darwin
goarch: arm64
pkg: github.com/epk/mmap-rpc/example/e2e
cpu: Apple M1 Pro
BenchmarkMMAPRPC-8           452           2552370 ns/op        11196784 B/op        107 allocs/op
BenchmarkMMAPRPC-8           476           2539737 ns/op        11196782 B/op        107 allocs/op
BenchmarkMMAPRPC-8           466           2552755 ns/op        11196794 B/op        107 allocs/op
BenchmarkMMAPRPC-8           429           2663084 ns/op        11196773 B/op        107 allocs/op
BenchmarkMMAPRPC-8           358           3265810 ns/op        11196773 B/op        107 allocs/op
BenchmarkMMAPRPC-8           411           2821651 ns/op        11196774 B/op        107 allocs/op
BenchmarkMMAPRPC-8           428           2760478 ns/op        11196772 B/op        107 allocs/op
BenchmarkMMAPRPC-8           360           2893714 ns/op        11196772 B/op        107 allocs/op
BenchmarkMMAPRPC-8           302           3618001 ns/op        11196773 B/op        107 allocs/op
BenchmarkMMAPRPC-8           338           3475888 ns/op        11196772 B/op        107 allocs/op
PASS
ok      github.com/epk/mmap-rpc/example/e2e     50.946s
```

vs gRPC over Unix Domain Socket

```
 go test -benchmem -run=^$ -bench ^BenchmarkGRPC$ github.com/epk/mmap-rpc/example/e2e  -count=10 | tee grpc.txt
goos: darwin
goarch: arm64
pkg: github.com/epk/mmap-rpc/example/e2e
cpu: Apple M1 Pro
BenchmarkGRPC-8               69          16425196 ns/op        12767883 B/op       1970 allocs/op
BenchmarkGRPC-8               68          16168973 ns/op        12302784 B/op       1952 allocs/op
BenchmarkGRPC-8               72          15714488 ns/op        12428810 B/op       1887 allocs/op
BenchmarkGRPC-8               70          16138236 ns/op        11979911 B/op       1841 allocs/op
BenchmarkGRPC-8               67          16653044 ns/op        12304689 B/op       1815 allocs/op
BenchmarkGRPC-8               69          15878616 ns/op        12808471 B/op       1884 allocs/op
BenchmarkGRPC-8               72          15830256 ns/op        12387684 B/op       1903 allocs/op
BenchmarkGRPC-8               70          15857945 ns/op        12731386 B/op       1794 allocs/op
BenchmarkGRPC-8               74          15383021 ns/op        13407783 B/op       1765 allocs/op
BenchmarkGRPC-8               70          16025252 ns/op        11849966 B/op       1888 allocs/op
PASS
ok      github.com/epk/mmap-rpc/example/e2e     47.427s
```

Even though this is merely a PoC and not optimized, it is already faster than gRPC over Unix Domain Socket when it comes to transferring large payloads.
