package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/google/uuid"
	"github.com/tysonmote/gommap"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/epk/mmap-rpc/gen/api"
	"github.com/epk/mmap-rpc/pkg/netstringconn"
)

type HandlerFunc func(ctx context.Context, data []byte) ([]byte, error)

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
		if err := s.receiveAndSend(nsConn); err != nil {
			if errors.Is(err, net.ErrClosed) || errors.Is(err, io.EOF) {
				// Connection closed or EOF reached, exit gracefully
				return
			}
			if errors.Is(err, syscall.EPIPE) || errors.Is(err, syscall.ECONNRESET) {
				// Client disconnected, exit gracefully
				return
			}
			log.Printf("Error handling request: %v\n", err)
			return // Exit the loop on any error
		}
	}
}

func (s *Server) receiveAndSend(w *netstringconn.NetstringConn) error {
	msg, err := w.Read()
	if err != nil {
		return fmt.Errorf("failed to read request: %w", err)
	}

	request := &anypb.Any{}
	if err := proto.Unmarshal(msg, request); err != nil {
		return fmt.Errorf("failed to unmarshal request: %w", err)
	}

	var response proto.Message

	switch request.TypeUrl {
	case "type.googleapis.com" + "/" + string(proto.MessageName(&api.ConnectRequest{})):
		response = s.handleConnect()
	case "type.googleapis.com" + "/" + string(proto.MessageName(&api.DisconnectRequest{})):
		typedRequest := &api.DisconnectRequest{}
		if err := anypb.UnmarshalTo(request, typedRequest, proto.UnmarshalOptions{}); err != nil {
			return fmt.Errorf("failed to unmarshal disconnect request: %w", err)
		}
		s.handleDisconnect(typedRequest.GetConnectionId())
		response = &api.Empty{}
	case "type.googleapis.com" + "/" + string(proto.MessageName(&api.RPCRequest{})):
		typedRequest := &api.RPCRequest{}
		if err := anypb.UnmarshalTo(request, typedRequest, proto.UnmarshalOptions{}); err != nil {
			return fmt.Errorf("failed to unmarshal data request: %w", err)
		}
		response = s.handleData(typedRequest)
	default:
		return fmt.Errorf("unknown request typeUrl: %s", request.TypeUrl)
	}

	responseData, err := proto.Marshal(response)
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}

	if err := w.Write(responseData); err != nil {
		return fmt.Errorf("failed to write response: %w", err)
	}

	return nil
}

func (s *Server) handleConnect() *api.ConnectResponse {
	connID := uuid.New().String()
	mmapFilename := filepath.Join(s.mmapFilePrefix + connID + ".mmap")

	file, err := os.Create(mmapFilename)
	if err != nil {
		log.Printf("[Connection ID: %s] failed to create mmap file: %v\n", connID, err)
		return &api.ConnectResponse{Error: err.Error()}
	}

	if err := file.Truncate(mmapFileSize); err != nil {
		log.Printf("[Connection ID: %s] failed to truncate mmap file: %v\n", connID, err)
		file.Close()
		return &api.ConnectResponse{Error: err.Error()}
	}

	mmap, err := gommap.Map(file.Fd(), gommap.PROT_READ|gommap.PROT_WRITE, gommap.MAP_SHARED)
	if err != nil {
		log.Printf("[Connection ID: %s] failed to mmap file: %v\n", connID, err)
		file.Close()
		return &api.ConnectResponse{Error: err.Error()}
	}

	conn := &Connection{
		id:       connID,
		mmapFile: file,
		mmap:     mmap,
	}

	s.connections.Store(connID, conn)

	return &api.ConnectResponse{
		ConnectionId: connID,
		MmapFilename: mmapFilename,
	}
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

func (s *Server) RegisterHandler(methodName string, handler HandlerFunc) {
	s.implsStubs.Store(methodName, handler)
}

func (s *Server) handleData(req *api.RPCRequest) *api.RPCResponse {
	response := &api.RPCResponse{
		ConnectionId:             req.ConnectionId,
		FullyQualifiedMethodName: req.FullyQualifiedMethodName,
		Size:                     0,
	}

	connInterface, ok := s.connections.Load(req.ConnectionId)
	if !ok {
		response.Error = fmt.Sprintf("connection not found: %s", req.ConnectionId)
		log.Printf("[Connection ID: %s] %s\n", req.ConnectionId, response.Error)
		return response
	}
	conn := connInterface.(*Connection)

	handlerInterface, ok := s.implsStubs.Load(req.FullyQualifiedMethodName)
	if !ok {
		response.Error = fmt.Sprintf("method not found: %s", req.FullyQualifiedMethodName)
		log.Printf("[Connection ID: %s] %s\n", conn.id, response.Error)
		return response
	}

	handler, ok := handlerInterface.(HandlerFunc)
	if !ok {
		response.Error = fmt.Sprintf("invalid handler for method: %s", req.FullyQualifiedMethodName)
		log.Printf("[Connection ID: %s] %s\n", conn.id, response.Error)
		return response
	}

	data := conn.mmap[:req.Size]
	out, err := handler(context.Background(), data)
	if err != nil {
		response.Error = fmt.Sprintf("handler error: %v", err)
		log.Printf("[Connection ID: %s] %s\n", conn.id, response.Error)
		return response
	}

	writeLimit := copy(conn.mmap[:len(out)], out)
	response.Size = uint64(writeLimit)

	return response
}
