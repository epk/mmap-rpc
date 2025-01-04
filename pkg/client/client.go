package client

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"sync"
	"time"

	"github.com/tysonmote/gommap"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/epk/mmap-rpc/gen/api"
	"github.com/epk/mmap-rpc/pkg/netstringconn"
)

// Client represents an RPC client with a pool of connections
type Client struct {
	socketPath string
	pool       chan *connection
	mu         sync.Mutex
}

// connection represents a single RPC connection
type connection struct {
	conn         *netstringconn.NetstringConn
	connectionID string
	mmapFile     *os.File
	mmap         gommap.MMap
	inUse        bool
}

// ClientOptions contains configuration for the client
type ClientOptions struct {
	PoolSize    int
	DialTimeout time.Duration
}

// DefaultClientOptions provides sensible defaults
var DefaultClientOptions = ClientOptions{
	PoolSize:    32,
	DialTimeout: time.Second,
}

// NewClient creates a new Client instance with a connection pool
func NewClient(socketPath string, opts ClientOptions) (*Client, error) {
	client := &Client{
		socketPath: socketPath,
		pool:       make(chan *connection, opts.PoolSize),
	}

	// Initialize the connection pool
	for i := 0; i < opts.PoolSize; i++ {
		conn, err := client.createConnection(opts.DialTimeout)
		if err != nil {
			// Clean up any connections we've already created
			client.Close()
			return nil, fmt.Errorf("failed to initialize connection pool: %w", err)
		}
		client.pool <- conn
	}

	return client, nil
}

// createConnection establishes a new connection to the server
func (c *Client) createConnection(timeout time.Duration) (*connection, error) {
	// Use net.DialTimeout for unix socket
	conn, err := net.DialTimeout("unix", c.socketPath, timeout)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to server: %w", err)
	}

	nsConn := netstringconn.NewNetstringConn(conn)
	connection := &connection{
		conn: nsConn,
	}

	if err := connection.connect(); err != nil {
		nsConn.Close()
		return nil, err
	}

	return connection, nil
}

// connect initializes the connection with the server and sets up the memory-mapped file
func (c *connection) connect() error {
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

// setupMmap sets up the memory-mapped file for data transfer
func (c *connection) setupMmap(filename string) error {
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

// Close closes all connections in the pool
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var lastErr error
	// Close all connections in the pool
	for {
		select {
		case conn := <-c.pool:
			if err := conn.close(); err != nil && lastErr == nil {
				lastErr = err
			}
		default:
			close(c.pool)
			return lastErr
		}
	}
}

// close terminates a single connection and cleans up resources
func (c *connection) close() error {
	var closeErr error

	if c.mmapFile != nil {
		if err := c.mmap.Sync(gommap.MS_SYNC); err != nil {
			log.Printf("[Connection ID: %s] failed to sync mmap: %v\n", c.connectionID, err)
		}

		if err := c.mmapFile.Close(); err != nil {
			closeErr = fmt.Errorf("failed to close mmap file: %w", err)
		}
	}

	disconnectRequest := api.DisconnectRequest_builder{
		ConnectionId: proto.String(c.connectionID),
	}.Build()

	if err := c.sendRequest(disconnectRequest); err != nil {
		if closeErr != nil {
			closeErr = fmt.Errorf("%v; failed to send disconnect request: %w", closeErr, err)
		} else {
			closeErr = fmt.Errorf("failed to send disconnect request: %w", err)
		}
	}

	if err := c.conn.Close(); err != nil {
		if closeErr != nil {
			closeErr = fmt.Errorf("%v; failed to close connection: %w", closeErr, err)
		} else {
			closeErr = fmt.Errorf("failed to close connection: %w", err)
		}
	}

	return closeErr
}

// Invoke sends an RPC request to the server and receives the response
func (c *Client) Invoke(ctx context.Context, method string, in, out proto.Message) error {
	// Get a connection from the pool
	conn := <-c.pool
	defer func() {
		// Return the connection to the pool
		c.pool <- conn
	}()

	// Lock the mmap for this operation
	if err := conn.mmap.Lock(); err != nil {
		return fmt.Errorf("failed to lock mmap: %w", err)
	}
	defer conn.mmap.Unlock()

	mo := proto.MarshalOptions{}
	inBytes, err := mo.MarshalAppend(conn.mmap[:0], in)
	if err != nil {
		return fmt.Errorf("failed to marshal input: %w", err)
	}

	rpcRequest := api.RPCRequest_builder{
		ConnectionId:             proto.String(conn.connectionID),
		FullyQualifiedMethodName: proto.String(method),
		Size:                     proto.Uint64(uint64(len(inBytes))),
	}.Build()

	rpcResponse := &api.RPCResponse{}
	if err := conn.sendAndReceive(rpcRequest, rpcResponse); err != nil {
		return fmt.Errorf("failed to invoke method %s: %w", method, err)
	}

	data := conn.mmap[:rpcResponse.GetSize()]
	return proto.Unmarshal(data, out)
}

// sendAndReceive sends a request and receives a response
func (c *connection) sendAndReceive(req, resp proto.Message) error {
	if err := c.sendRequest(req); err != nil {
		return err
	}

	respbuf, err := c.conn.Read()
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	return proto.Unmarshal(respbuf, resp)
}

// sendRequest converts the message to anypb and sends it to the server
func (c *connection) sendRequest(msg proto.Message) error {
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
