package client

import (
	"context"
	"fmt"
	"net"
	"os"

	"github.com/tysonmote/gommap"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/epk/mmap-rpc/gen/api"
	"github.com/epk/mmap-rpc/pkg/netstringconn"
)

// Client represents an RPC client using memory-mapped files for data transfer.
type Client struct {
	conn         *netstringconn.NetstringConn
	connectionID string
	mmapFile     *os.File
	mmap         gommap.MMap
}

// NewClient creates a new Client instance and establishes a connection to the server.
func NewClient(socketPath string) (*Client, error) {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to server: %w", err)
	}

	return &Client{
		conn: netstringconn.NewNetstringConn(conn),
	}, nil
}

// Connect initializes the connection with the server and sets up the memory-mapped file.
func (c *Client) Connect() error {
	connectRequest := &api.ConnectRequest{}
	connectResponse := &api.ConnectResponse{}

	if err := c.sendAndReceive(connectRequest, connectResponse); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	c.connectionID = connectResponse.ConnectionId
	if err := c.setupMmap(connectResponse.MmapFilename); err != nil {
		return fmt.Errorf("failed to setup mmap: %w", err)
	}
	return nil
}

// setupMmap sets up the memory-mapped file for data transfer.
func (c *Client) setupMmap(filename string) error {
	file, err := os.OpenFile(filename, os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("failed to open mmap file: %w", err)
	}

	mmap, err := gommap.Map(file.Fd(), gommap.PROT_READ|gommap.PROT_WRITE, gommap.MAP_SHARED)
	if err != nil {
		file.Close()
		return fmt.Errorf("failed to mmap file: %w", err)
	}

	c.mmap = mmap
	c.mmapFile = file

	return nil
}

// Close terminates the connection with the server and cleans up resources.
func (c *Client) Close() error {
	if c.mmapFile != nil {
		if err := c.mmapFile.Close(); err != nil {
			return fmt.Errorf("failed to close mmap file: %w", err)
		}
	}

	disconnectRequest := &api.DisconnectRequest{
		ConnectionId: c.connectionID,
	}
	if err := c.sendRequest(disconnectRequest); err != nil {
		return fmt.Errorf("failed to send disconnect request: %w", err)
	}

	return c.conn.Close()
}

// Invoke sends an RPC request to the server and receives the response.
func (c *Client) Invoke(ctx context.Context, method string, in, out proto.Message) error {
	inBytes, err := proto.Marshal(in)
	if err != nil {
		return fmt.Errorf("failed to marshal input: %w", err)
	}

	writeLimit := copy(c.mmap, inBytes)

	rpcRequest := &api.RPCRequest{
		ConnectionId:             c.connectionID,
		FullyQualifiedMethodName: method,
		Size:                     uint64(writeLimit),
	}
	rpcResponse := &api.RPCResponse{}

	if err := c.sendAndReceive(rpcRequest, rpcResponse); err != nil {
		return fmt.Errorf("failed to invoke method %s: %w", method, err)
	}

	data := c.mmap[:rpcResponse.Size]
	return proto.Unmarshal(data, out)
}

// sendAndReceive sends a request and receives a response.
func (c *Client) sendAndReceive(req, resp proto.Message) error {
	if err := c.sendRequest(req); err != nil {
		return err
	}

	respbuf, err := c.conn.Read()
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	return proto.Unmarshal(respbuf, resp)
}

// sendRequest converts the message to anypb and sends it to the server.
func (c *Client) sendRequest(msg proto.Message) error {
	any, err := anypb.New(msg)
	if err != nil {
		return fmt.Errorf("failed to create any: %w", err)
	}

	bytes, err := proto.Marshal(any)
	if err != nil {
		return fmt.Errorf("failed to marshal any: %w", err)
	}

	return c.conn.Write(bytes)
}
