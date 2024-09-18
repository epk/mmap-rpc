package cache

import (
	context "context"

	"google.golang.org/protobuf/proto"

	"github.com/epk/mmap-rpc/pkg/client"
	"github.com/epk/mmap-rpc/pkg/server"
)

const (
	_Cache_Get_FullMethodName = "/cache.Cache/Get"
	_Cache_Set_FullMethodName = "/cache.Cache/Set"
)

// MmapRPCCacheClient is the client API for Cache service.
type MmapRPCCacheClient interface {
	Get(ctx context.Context, in *GetRequest) (*GetResponse, error)
	Set(ctx context.Context, in *SetRequest) (*SetResponse, error)
}

type mmapRPCCacheClient struct {
	client *client.Client
}

func (c *mmapRPCCacheClient) Get(ctx context.Context, in *GetRequest) (*GetResponse, error) {
	out := &GetResponse{}
	if err := c.client.Invoke(ctx, _Cache_Get_FullMethodName, in, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *mmapRPCCacheClient) Set(ctx context.Context, in *SetRequest) (*SetResponse, error) {
	out := &SetResponse{}
	if err := c.client.Invoke(ctx, _Cache_Set_FullMethodName, in, out); err != nil {
		return nil, err
	}
	return out, nil
}

// NewMmapRPCCacheClient creates a new MmapRPCCacheClient
func NewMmapRPCCacheClient(client *client.Client) MmapRPCCacheClient {
	return &mmapRPCCacheClient{
		client: client,
	}
}

// MmapRPCCacheServer is the server API for Cache service.
type MmapRPCCacheServer interface {
	Get(context.Context, *GetRequest) (*GetResponse, error)
	Set(context.Context, *SetRequest) (*SetResponse, error)
}

// RegisterMmapRPCCacheServer registers the MmapRPCCacheServer with the given server.
func RegisterMmapRPCCacheServer(s *server.Server, srv MmapRPCCacheServer) {
	s.RegisterHandler(_Cache_Get_FullMethodName, func(ctx context.Context, data []byte) ([]byte, error) {
		return handleRequest(ctx, data, srv.Get, &GetRequest{})
	})

	s.RegisterHandler(_Cache_Set_FullMethodName, func(ctx context.Context, data []byte) ([]byte, error) {
		return handleRequest(ctx, data, srv.Set, &SetRequest{})
	})
}

// handleRequest is a helper function to reduce code duplication in RegisterMmapRPCCacheServer
func handleRequest[Req, Resp proto.Message](
	ctx context.Context,
	data []byte,
	handler func(context.Context, Req) (Resp, error),
	req Req,
) ([]byte, error) {
	if err := proto.Unmarshal(data, req); err != nil {
		return nil, err
	}
	resp, err := handler(ctx, req)
	if err != nil {
		return nil, err
	}
	return proto.Marshal(resp)
}
