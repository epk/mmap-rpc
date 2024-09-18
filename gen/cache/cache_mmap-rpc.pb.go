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

func NewMmapRPCCacheClient(client *client.Client) MmapRPCCacheClient {
	return &mmapRPCCacheClient{
		client: client,
	}
}

type MmapRPCCacheServer interface {
	Get(context.Context, *GetRequest) (*GetResponse, error)
	Set(context.Context, *SetRequest) (*SetResponse, error)
}

func RegisterMmapRPCCacheServer(s *server.Server, srv MmapRPCCacheServer) {
	s.RegisterImplStub(_Cache_Get_FullMethodName, func(in []byte) ([]byte, error) {
		req := &GetRequest{}
		if err := proto.Unmarshal(in, req); err != nil {
			return nil, err
		}

		out, err := srv.Get(context.Background(), req)
		if err != nil {
			return nil, err
		}

		return proto.Marshal(out)
	})

	s.RegisterImplStub(_Cache_Set_FullMethodName, func(in []byte) ([]byte, error) {
		req := &SetRequest{}
		if err := proto.Unmarshal(in, req); err != nil {
			return nil, err
		}

		out, err := srv.Set(context.Background(), req)
		if err != nil {
			return nil, err
		}

		return proto.Marshal(out)
	})
}
