package server

import (
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/tysonmote/gommap"

	"github.com/epk/mmap-rpc/pkg/netstringconn"
)

type Connection struct {
	id       string
	mmapFile *os.File
	mmap     gommap.MMap
}

type Server struct {
	listener       net.Listener
	connections    sync.Map
	mmapFilePrefix string

	implsStubs sync.Map
}

var mmapFileSize int64 = 1 * 1024 * 1024 // 1MB

func (s *Server) ListenAndServe(socketPath, mmapFilePrefix string) error {
	s.mmapFilePrefix = mmapFilePrefix

	if err := os.RemoveAll(socketPath); err != nil {
		return fmt.Errorf("failed to remove existing socket: %w", err)
	}

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return fmt.Errorf("failed to listen on socket: %w", err)
	}
	s.listener = listener
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}

		go s.handleConnection(conn)
	}
}

func (s *Server) Close() {
	// Close all connections
	s.connections.Range(
		func(key, value interface{}) bool {
			conn := value.(*Connection)
			s.handleDisconnect(conn.id)
			return true
		},
	)

	s.listener.Close()
}

func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()

	nsConn := netstringconn.NewNetstringConn(conn)

	for {
		msg, err := nsConn.Read()
		if err != nil {
			// handle EOF/ closed connection
			if errors.Is(err, net.ErrClosed) {
				return
			}

			return
		}

		parts := strings.Split(string(msg), ",")

		switch parts[0] {
		case "CONNECT":
			connID, mmapFilename, err := s.handleConnect()
			if err != nil {
				// handleConnect already logs the error
				continue
			}

			response := fmt.Sprintf("CONNECTED,%s,%s", connID, mmapFilename)
			if err := nsConn.Write([]byte(response)); err != nil {
				log.Printf("[Connection ID: %s] failed to write response: %v\n", connID, err)
			}
		case "DISCONNECT":
			if len(parts) != 2 {
				log.Printf("Invalid DISCONNECT message")
				return
			}

			connID := parts[1]
			s.handleDisconnect(connID)
			if err := nsConn.Close(); err != nil {
				log.Printf("[Connection ID: %s] failed to close connection: %v\n", connID, err)
			}
			// DATA,<connID>,FQMN,<read-limit>
		case "DATA":
			if len(parts) != 4 {
				log.Printf("Invalid DATA message")
				return
			}

			connID := parts[1]
			fullyQualifiedMethodName := parts[2]
			readLimit := parts[3]
			readLimitInt, _ := strconv.Atoi(readLimit)
			c, ok := s.connections.Load(connID)
			if !ok {
				log.Printf("[Connection ID: %s] connection not found\n", connID)
				return
			}
			cc := c.(*Connection)

			s.handleData(nsConn, cc, fullyQualifiedMethodName, readLimitInt)
		}
	}
}

func (s *Server) handleConnect() (string, string, error) {
	connID := uuid.New().String()
	mmapFilename := filepath.Join(s.mmapFilePrefix + connID + ".mmap")

	file, err := os.Create(mmapFilename)
	if err != nil {
		log.Printf("[Connection ID: %s] failed to create mmap file: %v\n", connID, err)
		return "", "", fmt.Errorf("failed to create mmap file: %w", err)
	}

	if err := file.Truncate(mmapFileSize); err != nil {
		log.Printf("[Connection ID: %s] failed to truncate mmap file: %v\n", connID, err)
		file.Close()
		return "", "", fmt.Errorf("failed to truncate mmap file: %w", err)
	}

	mmap, err := gommap.Map(file.Fd(), gommap.PROT_READ|gommap.PROT_WRITE, gommap.MAP_SHARED)
	if err != nil {
		log.Printf("[Connection ID: %s] failed to mmap file: %v\n", connID, err)
		file.Close()
		return "", "", fmt.Errorf("failed to mmap file: %w", err)
	}

	conn := &Connection{
		id:       connID,
		mmapFile: file,
		mmap:     mmap,
	}

	s.connections.Store(connID, conn)
	return connID, mmapFilename, nil
}

func (s *Server) handleDisconnect(connID string) {
	connInterface, ok := s.connections.Load(connID)
	if !ok {
		return
	}
	conn := connInterface.(*Connection)

	if err := conn.mmapFile.Close(); err != nil {
		log.Printf("[Connection ID: %s] failed to close mmap file: %v\n", connID, err)
	}

	if err := os.Remove(conn.mmapFile.Name()); err != nil {
		log.Printf("[Connection ID: %s] failed to remove mmap file: %v\n", connID, err)
	}

	s.connections.Delete(connID)
}

func (s *Server) RegisterImplStub(fullyQualifiedMethodName string, implStub interface{}) {
	s.implsStubs.Store(fullyQualifiedMethodName, implStub)
}

func (s *Server) handleData(w *netstringconn.NetstringConn, conn *Connection, fullyQualifiedMethodName string, readLimit int) {
	// we read data from the mmap file
	data := conn.mmap[:readLimit]

	implStubInterface, ok := s.implsStubs.Load(fullyQualifiedMethodName)
	if !ok {
		log.Printf("[Connection ID: %s] method not found: %s\n", conn.id, fullyQualifiedMethodName)
	}

	implStub := implStubInterface.(func(in []byte) ([]byte, error))
	out, err := implStub(data)
	if err != nil {
		log.Printf("[Connection ID: %s] failed to process data: %v\n", conn.id, err)
		return
	}

	// we write data back to the mmap file
	writeLimit := copy(conn.mmap[:len(out)], out)
	// respond to the client
	payload := fmt.Sprintf("DATA,%s,%s,%d", conn.id, fullyQualifiedMethodName, writeLimit)
	if err := w.Write([]byte(payload)); err != nil {
		log.Printf("[Connection ID: %s] failed to write response: %v\n", conn.id, err)
	}
}
