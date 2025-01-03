package client

import (
	"context"
	"fmt"
	"log"
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

	c.connectionID = connectResponse.GetConnectionId()
	if err := c.setupMmap(connectResponse.GetMmapFilename()); err != nil {
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

	disconnectRequest := api.DisconnectRequest_builder{
		ConnectionId: proto.String(c.connectionID),
	}.Build()

	if err := c.sendRequest(disconnectRequest); err != nil {
		return fmt.Errorf("failed to send disconnect request: %w", err)
	}

	return c.conn.Close()
}

// Invoke sends an RPC request to the server and receives the response.
func (c *Client) Invoke(ctx context.Context, method string, in, out proto.Message) error {
	if err := c.mmap.Lock(); err != nil {
		return fmt.Errorf("failed to lock mmap: %w", err)
	}
	defer func() {
		if err := c.mmap.Unlock(); err != nil {
			log.Printf("[Connection ID: %s] failed to unlock mmap: %v\n", c.connectionID, err)
		}

		if err := c.mmap.Sync(gommap.MS_SYNC); err != nil {
			log.Printf("[Connection ID: %s] failed to sync mmap: %v\n", c.connectionID, err)
		}
	}()

	mo := proto.MarshalOptions{}
	inBytes, err := mo.MarshalAppend(c.mmap[:0], in)
	if err != nil {
		return fmt.Errorf("failed to marshal input: %w", err)
	}

	rpcRequest := api.RPCRequest_builder{
		ConnectionId:             proto.String(c.connectionID),
		FullyQualifiedMethodName: proto.String(method),
		Size:                     proto.Uint64(uint64(len(inBytes))),
	}.Build()

	rpcResponse := &api.RPCResponse{}
	if err := c.sendAndReceive(rpcRequest, rpcResponse); err != nil {
		return fmt.Errorf("failed to invoke method %s: %w", method, err)
	}

	data := c.mmap[:rpcResponse.GetSize()]
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
