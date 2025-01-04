package e2e_test

import (
	"context"
	"testing"
	"time"

	cache "github.com/epk/mmap-rpc/example/gen/api"
	"github.com/epk/mmap-rpc/pkg/client"
	"github.com/epk/mmap-rpc/pkg/server"
	"google.golang.org/protobuf/proto"
)

func Test_E2E(t *testing.T) {
	filePath := t.TempDir()
	sockPath := filePath + "/server.sock"

	s := &server.Server{}
	cache.RegisterMmapRPCCacheServer(s, &srv{t: t})

	go func() {
		if err := s.ListenAndServe(sockPath, filePath); err != nil {
			t.Error(err, "failed to start server")
		}
	}()

	var c *client.Client
	var err error
	maxAttempts := 10

	for i := 0; i < maxAttempts; i++ {
		c, err = client.NewClient(sockPath)
		if err == nil {
			break
		}

		if i == maxAttempts-1 {
			t.Fatal(err, "failed to create client after", maxAttempts, "attempts")
		}

		t.Log("failed to create client, retrying...")
		time.Sleep(100 * time.Millisecond)
	}

	if err := c.Connect(); err != nil {
		t.Fatal(err, "failed to connect to server")
	}

	t.Run("Set", func(t *testing.T) {
		cc := cache.NewMmapRPCCacheClient(c)
		req := cache.SetRequest_builder{
			Key:   proto.String("key"),
			Value: proto.String("value"),
		}.Build()

		if _, err := cc.Set(context.Background(), req); err != nil {
			t.Fatal(err, "failed to set key")
		}
	})

	t.Run("Get", func(t *testing.T) {
		cc := cache.NewMmapRPCCacheClient(c)
		req := cache.GetRequest_builder{
			Key: proto.String("key"),
		}.Build()

		resp, err := cc.Get(context.Background(), req)
		if err != nil {
			t.Fatal(err, "failed to get key")
		}

		if resp.GetValue() != "value" {
			t.Fatal("expected value to be 'value', got", resp.GetValue())
		}
	})

}

type srv struct {
	value string
	t     *testing.T
}

func (s *srv) Get(ctx context.Context, in *cache.GetRequest) (*cache.GetResponse, error) {
	s.t.Log("[server] Get request for key:", in.GetKey())

	resp := cache.GetResponse_builder{
		Value: proto.String(s.value),
		Found: proto.Bool(true),
	}.Build()

	return resp, nil
}
func (s *srv) Set(ctx context.Context, in *cache.SetRequest) (*cache.SetResponse, error) {
	s.t.Log("[server] Set request for key:", in.GetKey(), "value:", in.GetValue())

	s.value = in.GetValue()

	resp := cache.SetResponse_builder{
		Success: proto.Bool(true),
	}.Build()

	return resp, nil
}
