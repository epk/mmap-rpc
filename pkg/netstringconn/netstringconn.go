package netstringconn

import (
	"bufio"
	"net"

	"github.com/kyrylo/netstring"
)

// NetstringConn is a wrapper around net.Conn that reads and writes data in netstring format.
type NetstringConn struct {
	conn   net.Conn
	reader *bufio.Reader
}

func NewNetstringConn(conn net.Conn) *NetstringConn {
	return &NetstringConn{
		conn:   conn,
		reader: bufio.NewReader(conn),
	}
}

func (nc *NetstringConn) Read() ([]byte, error) {
	return netstring.Parse(nc.reader)
}

func (nc *NetstringConn) Write(data []byte) error {
	_, err := nc.conn.Write(netstring.Pack(data))
	return err
}

func (nc *NetstringConn) Close() error {
	return nc.conn.Close()
}
