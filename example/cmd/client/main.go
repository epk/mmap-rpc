package main

import (
	"context"
	"fmt"

	cache "github.com/epk/mmap-rpc/example/gen/api"
	"github.com/epk/mmap-rpc/pkg/client"
	"google.golang.org/protobuf/proto"
)

func main() {
	c, err := client.NewClient("/tmp/mmap/server.sock", client.DefaultClientOptions)
	if err != nil {
		panic(err)
	}
	defer c.Close()

	cc := cache.NewMmapRPCCacheClient(c)

	r, err := cc.Get(context.Background(), cache.GetRequest_builder{Key: proto.String("foo")}.Build())
	if err != nil {
		fmt.Println("[client] Get error:", err)
	} else {
		fmt.Printf("[client] Get response: %v\n", r)
	}

	rr, err := cc.Set(context.Background(),
		cache.SetRequest_builder{
			Key:   proto.String("foo"),
			Value: proto.String("bar"),
		}.Build(),
	)
	if err != nil {
		fmt.Println("[client] Set error:", err)
	} else {
		fmt.Println("[client] Set response:", rr)
	}

	rrr, err := cc.Get(context.Background(),
		cache.GetRequest_builder{
			Key: proto.String("foo"),
		}.Build(),
	)
	if err != nil {
		fmt.Println("[client] Get error:", err)
	} else {
		fmt.Printf("[client] Get response: %v\n", rrr)
	}
}
