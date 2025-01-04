package e2e_test

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	cache "github.com/epk/mmap-rpc/example/gen/api"
	"github.com/epk/mmap-rpc/pkg/client"
	"github.com/epk/mmap-rpc/pkg/server"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"
)

func Test_E2E(t *testing.T) {
	filePath := t.TempDir()
	sockPath := filePath + "/server.sock"

	s := &server.Server{}
	cache.RegisterMmapRPCCacheServer(s, &srv{})

	go func() {
		if err := s.ListenAndServe(sockPath, filePath); err != nil {
			t.Error(err, "failed to start server")
		}
	}()

	c, err := client.NewClient(sockPath, client.DefaultClientOptions)
	if err != nil {
		t.Fatal(err, "failed to create client")
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
	storage sync.Map
	cache.UnimplementedCacheServer
}

func (s *srv) Get(ctx context.Context, in *cache.GetRequest) (*cache.GetResponse, error) {
	if v, ok := s.storage.Load(in.GetKey()); ok {
		resp := cache.GetResponse_builder{
			Value: proto.String(v.(string)),
			Found: proto.Bool(true),
		}.Build()

		return resp, nil
	}

	// Not found
	resp := cache.GetResponse_builder{
		Found: proto.Bool(false),
	}.Build()
	return resp, nil
}

func (s *srv) Set(ctx context.Context, in *cache.SetRequest) (*cache.SetResponse, error) {
	s.storage.Store(in.GetKey(), in.GetValue())

	resp := cache.SetResponse_builder{
		Success: proto.Bool(true),
	}.Build()

	return resp, nil
}

func BenchmarkMMAPRPC(b *testing.B) {
	// Skip regular tests
	if testing.Short() {
		b.Skip("Skipping benchmark in short mode")
	}

	filePath := b.TempDir()
	sockPath := filePath + "/server.sock"

	s := &server.Server{}
	cache.RegisterMmapRPCCacheServer(s, &srv{})

	go func() {
		if err := s.ListenAndServe(sockPath, filePath); err != nil {
			b.Error(err, "failed to start server")
		}
	}()

	time.Sleep(1 * time.Second)

	c, err := client.NewClient(sockPath, client.DefaultClientOptions)
	if err != nil {
		b.Fatal(err, "failed to create client")
	}
	client := cache.NewMmapRPCCacheClient(c)

	// Generate 4MB payload
	payload := make([]byte, 4*1024*1024)
	_, err = rand.Read(payload)
	if err != nil {
		b.Fatal(err)
	}
	value := base64.StdEncoding.EncodeToString(payload)

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		key := fmt.Sprintf("key-%d", time.Now().UnixNano())

		for pb.Next() {
			// Set operation
			setReq := cache.SetRequest_builder{
				Key:   proto.String(key),
				Value: proto.String(value),
			}.Build()

			_, err := client.Set(context.Background(), setReq)
			if err != nil {
				b.Fatal("Set failed:", err)
			}

			// Get operation
			getReq := cache.GetRequest_builder{
				Key: proto.String(key),
			}.Build()

			resp, err := client.Get(context.Background(), getReq)
			if err != nil {
				b.Fatal("Get failed:", err)
			}

			if resp.GetValue() != value {
				b.Fatal("Value mismatch")
			}
		}
	})
}

func BenchmarkGRPC(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping benchmark in short mode")
	}

	maxSize := 8 * 1024 * 1024 // 8MB
	sockPath := b.TempDir() + "/grpc.sock"
	lis, err := net.Listen("unix", sockPath)
	if err != nil {
		b.Fatal(err)
	}

	// Create and start gRPC server
	s := grpc.NewServer(grpc.MaxRecvMsgSize(maxSize))
	cache.RegisterCacheServer(s, &srv{})
	go func() {
		if err := s.Serve(lis); err != nil {
			b.Error(err)
		}
	}()

	// Allow server to start
	time.Sleep(1 * time.Second)

	// Create gRPC client
	conn, err := grpc.NewClient(
		"unix://"+sockPath,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize((maxSize))),
		grpc.WithBlock(),
	)
	if err != nil {
		b.Fatal(err)
	}
	defer conn.Close()

	client := cache.NewCacheClient(conn)

	// Generate 4MB payload
	payload := make([]byte, 4*1024*1024)
	_, err = rand.Read(payload)
	if err != nil {
		b.Fatal(err)
	}
	value := base64.StdEncoding.EncodeToString(payload)

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		key := fmt.Sprintf("key-%d", time.Now().UnixNano())

		for pb.Next() {
			// Set operation
			setReq := cache.SetRequest_builder{
				Key:   proto.String(key),
				Value: proto.String(value),
			}.Build()

			_, err := client.Set(context.Background(), setReq)
			if err != nil {
				b.Fatal("Set failed:", err)
			}

			// Get operation
			getReq := cache.GetRequest_builder{
				Key: proto.String(key),
			}.Build()

			resp, err := client.Get(context.Background(), getReq)
			if err != nil {
				b.Fatal("Get failed:", err)
			}

			if resp.GetValue() != value {
				b.Fatal("Value mismatch")
			}
		}
	})
}
