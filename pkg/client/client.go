package client

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/epk/mmap-rpc/pkg/netstringconn"
	"github.com/tysonmote/gommap"
	"google.golang.org/protobuf/proto"
)

type Client struct {
	conn         *netstringconn.NetstringConn
	connectionID string
	mmapFile     *os.File
	mmap         gommap.MMap
}

func NewClient(socketPath string) (*Client, error) {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to server: %w", err)
	}

	return &Client{
		conn: netstringconn.NewNetstringConn(conn),
	}, nil
}

func (c *Client) Connect() error {
	err := c.conn.Write([]byte("CONNECT"))
	if err != nil {
		return fmt.Errorf("failed to write to server: %w", err)
	}

	response, err := c.conn.Read()
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Expected response format:
	// CONNECTED,834ad25c-683b-452a-b47c-649b036ce826,/tmp/834ad25c-683b-452a-b47c-649b036ce826.mmap
	parts := strings.Split(string(response), ",")
	if len(parts) != 3 || parts[0] != "CONNECTED" {
		return fmt.Errorf("invalid response: %s", response)
	}

	c.connectionID = parts[1]

	file, err := os.OpenFile(parts[2], os.O_RDWR, 0)
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

func (c *Client) Close() error {
	err := c.conn.Write([]byte("DISCONNECT" + "," + c.connectionID))
	if err != nil {
		return fmt.Errorf("failed to write to server: %w", err)
	}

	c.conn.Close()
	return nil
}

func (c *Client) Invoke(ctx context.Context, method string, in, out proto.Message) error {
	inBytes, err := proto.Marshal(in)
	if err != nil {
		return fmt.Errorf("failed to marshal input: %w", err)
	}
	writeLimit := copy(c.mmap[0:], inBytes)

	payload := fmt.Sprintf("DATA,%s,%s,%d", c.connectionID, method, writeLimit)
	err = c.conn.Write([]byte(payload))
	if err != nil {
		return fmt.Errorf("failed to write to server: %w", err)
	}

	response, err := c.conn.Read()
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	parts := strings.Split(string(response), ",")
	if len(parts) != 4 || parts[0] != "DATA" || parts[1] != c.connectionID {
		return fmt.Errorf("invalid response: %s", response)
	}

	readLimit := parts[3]
	readLimitInt, _ := strconv.Atoi(readLimit)
	data := c.mmap[0:readLimitInt]

	if err := proto.Unmarshal(data, out); err != nil {
		return fmt.Errorf("failed to unmarshal output: %w", err)
	}

	return nil
}
