package main

import (
	"context"
	"fmt"

	"github.com/epk/mmap-rpc/gen/cache"
	"github.com/epk/mmap-rpc/pkg/client"
)

func main() {
	c, err := client.NewClient("/tmp/mmap/server.sock")
	if err != nil {
		panic(err)
	}
	defer c.Close()

	if err := c.Connect(); err != nil {
		panic(err)
	}
	cc := cache.NewMmapRPCCacheClient(c)

	r, err := cc.Get(context.Background(), &cache.GetRequest{
		Key: "foo",
	})
	if err != nil {
		fmt.Println("[client] Get error:", err)
	} else {
		fmt.Printf("[client] Get response: %v\n", r)
	}

	rr, err := cc.Set(context.Background(), &cache.SetRequest{
		Key:   "foo",
		Value: "bar",
	})
	if err != nil {
		fmt.Println("[client] Set error:", err)
	} else {
		fmt.Println("[client] Set response:", rr)
	}

	rrr, err := cc.Get(context.Background(), &cache.GetRequest{
		Key: "foo",
	})
	if err != nil {
		fmt.Println("[client] Get error:", err)
	} else {
		fmt.Printf("[client] Get response: %v\n", rrr)
	}
}
