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


#### Codegen

Not yet implemented, but the reference output can be found in `gen/cache/cache_mmap-rpc.pb.go`. This code plugs into the client and server client libraries to abstract the protocol details from the user and provide a clean interface for making RPC calls (just like gRPC, twirp, etc.).

#### Example

Example usage of the client and server can be found in `cmd/client` and `cmd/server`.
