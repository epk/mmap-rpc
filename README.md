## mmap-rpc

A simple RPC framework based on memory-mapped files.

#### Protocol Specification

The mmap-rpc protocol uses netstring-encoded messages for the control flow between the client and server. The protocol defines the following message types and formats:

1. CONNECT
   - Client to Server: `CONNECT`
   - Server to Client: `CONNECTED,<connection_id>,<mmap_filename>`

2. DISCONNECT
   - Client to Server: `DISCONNECT,<connection_id>`
   - No response from server

3. DATA
   - Client to Server: `DATA,<connection_id>,<url>,<offset>`
   - Server to Client: `DATA,<connection_id>,<url>,<offset>`


Message Details:
1. CONNECT:
   - Initiated by the client to establish a connection.
   - The server responds with a unique connection ID and the filename of the memory-mapped file to be used for data transfer.
   - The client must store the connection ID and inlcude it in all subsequent messages.

2. DISCONNECT:
   - Sent by the client to end the connection.
   - The connection ID is included to identify the client.
   - The server closes the connection without sending a response.

3. DATA:
   - Used for making remote procedure calls.
   - The client sends the connection ID, the URL of the service/method to call, and the offset in the memory-mapped file where the request data is written.
   - The server responds with the connection ID and the offset where the response data is written in the memory-mapped file.


This protocol allows for efficient data transfer between the client and server using memory-mapped files while using small, fixed-size messages for control flow.

###### Why netstring

See https://cr.yp.to/proto/netstrings.txt

It is a simple, efficient, and easy-to-parse format that allows for variable-length messages without the need for delimiters.


#### Libraries

The reference client and server implemetations in `pkg/client` and `pkg/server` provide a pluggable interface for the client and server stubs to use.


#### Codegen

Not yet implemented, but the reference output can be found in `gen/proto/cache_mmap-rpc.pb.go`. This code plugs into the client and server client libraries to abstract the protocol details from the user and provide a clean interface for making RPC calls (just like gRPC, twirp, etc.).

#### Example

Example usage of the client and server can be found in `cmd/client` and `cmd/server`.
