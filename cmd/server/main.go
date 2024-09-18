package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/epk/mmap-rpc/gen/cache"
	"github.com/epk/mmap-rpc/pkg/server"
)

var value = "😄😄😄😄😄😄😄😄😄😄😄😄😄😄😄😄"

func main() {
	shutDown := make(chan os.Signal, 1)
	signal.Notify(shutDown, os.Interrupt)

	srv := server.Server{}

	cache.RegisterMmapRPCCacheServer(&srv, &stub{})
	go func() {
		if err := srv.ListenAndServe("/tmp/mmap/server.sock", "/tmp/mmap/"); err != nil {
			panic(err)
		}
	}()

	<-shutDown
	srv.Close()
}

var _ cache.MmapRPCCacheServer = (*stub)(nil)

type stub struct {
}

func (s *stub) Get(ctx context.Context, in *cache.GetRequest) (*cache.GetResponse, error) {
	fmt.Println("[server] Get request for key:", in.Key)

	return &cache.GetResponse{
		Value: value,
		Found: true,
	}, nil
}
func (s *stub) Set(ctx context.Context, in *cache.SetRequest) (*cache.SetResponse, error) {
	fmt.Println("[server] Set request for key:", in.Key, "value:", in.Value)

	value = in.Value
	return &cache.SetResponse{
		Success: true,
	}, nil
}
