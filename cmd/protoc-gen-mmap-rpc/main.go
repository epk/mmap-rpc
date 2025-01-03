package main

import (
	"fmt"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/types/pluginpb"
)

func main() {
	protogen.Options{}.Run(func(gen *protogen.Plugin) error {
		gen.SupportedFeatures = uint64(pluginpb.CodeGeneratorResponse_FEATURE_PROTO3_OPTIONAL)
		for _, f := range gen.Files {
			if !f.Generate {
				continue
			}
			generateFile(gen, f)
		}
		return nil
	})
}

func generateFile(gen *protogen.Plugin, file *protogen.File) {
	filename := file.GeneratedFilenamePrefix + "-rpc.pb.go"
	g := gen.NewGeneratedFile(filename, file.GoImportPath)

	g.P("package ", file.GoPackageName)
	g.P()

	// Generate imports
	g.P("import (")
	g.P(`    context "context"`)
	g.P(`    "google.golang.org/protobuf/proto"`)
	g.P(`    "github.com/epk/mmap-rpc/pkg/client"`)
	g.P(`    "github.com/epk/mmap-rpc/pkg/server"`)
	g.P(")")
	g.P()

	// Generate services
	for _, service := range file.Services {
		generateService(g, service)
	}
}

func generateService(g *protogen.GeneratedFile, service *protogen.Service) {
	// Generate method constants
	g.P("const (")
	for _, method := range service.Methods {
		g.P(fmt.Sprintf("    _%s_%s_FullMethodName = \"/%s.%s/%s\"",
			service.GoName, method.GoName,
			service.Desc.Parent().Name(), service.GoName, method.GoName))
	}
	g.P(")")
	g.P()

	// Generate client interface
	g.P("// MmapRPC", service.GoName, "Client is the client API for ", service.GoName, " service.")
	g.P("type MmapRPC", service.GoName, "Client interface {")
	for _, method := range service.Methods {
		g.P("    ", method.GoName, "(ctx context.Context, in *", method.Input.GoIdent.GoName, ") (*", method.Output.GoIdent.GoName, ", error)")
	}
	g.P("}")
	g.P()

	// Generate client implementation
	g.P("type mmapRPC", service.GoName, "Client struct {")
	g.P("    client *client.Client")
	g.P("}")
	g.P()

	// Generate client methods
	for _, method := range service.Methods {
		g.P("func (c *mmapRPC", service.GoName, "Client) ", method.GoName, "(ctx context.Context, in *", method.Input.GoIdent.GoName, ") (*", method.Output.GoIdent.GoName, ", error) {")
		g.P("    out := &", method.Output.GoIdent.GoName, "{}")
		g.P("    if err := c.client.Invoke(ctx, _", service.GoName, "_", method.GoName, "_FullMethodName, in, out); err != nil {")
		g.P("        return nil, err")
		g.P("    }")
		g.P("    return out, nil")
		g.P("}")
		g.P()
	}

	// Generate client constructor
	g.P("// NewMmapRPC", service.GoName, "Client creates a new MmapRPC", service.GoName, "Client")
	g.P("func NewMmapRPC", service.GoName, "Client(client *client.Client) MmapRPC", service.GoName, "Client {")
	g.P("    return &mmapRPC", service.GoName, "Client{")
	g.P("        client: client,")
	g.P("    }")
	g.P("}")
	g.P()

	// Generate server interface
	g.P("// MmapRPC", service.GoName, "Server is the server API for ", service.GoName, " service.")
	g.P("type MmapRPC", service.GoName, "Server interface {")
	for _, method := range service.Methods {
		g.P("    ", method.GoName, "(context.Context, *", method.Input.GoIdent.GoName, ") (*", method.Output.GoIdent.GoName, ", error)")
	}
	g.P("}")
	g.P()

	// Generate server registration
	g.P("// RegisterMmapRPC", service.GoName, "Server registers the MmapRPC", service.GoName, "Server with the given server.")
	g.P("func RegisterMmapRPC", service.GoName, "Server(s *server.Server, srv MmapRPC", service.GoName, "Server) {")
	for _, method := range service.Methods {
		g.P("    s.RegisterHandler(_", service.GoName, "_", method.GoName, "_FullMethodName, func(ctx context.Context, data []byte) ([]byte, error) {")
		g.P("        return handleRequest(ctx, data, srv.", method.GoName, ", &", method.Input.GoIdent.GoName, "{})")
		g.P("    })")
		g.P()
	}
	g.P("}")
	g.P()

	// Generate handleRequest helper
	g.P("// handleRequest is a helper function to reduce code duplication in RegisterMmapRPC", service.GoName, "Server")
	g.P("func handleRequest[Req, Resp proto.Message](")
	g.P("    ctx context.Context,")
	g.P("    data []byte,")
	g.P("    handler func(context.Context, Req) (Resp, error),")
	g.P("    req Req,")
	g.P(") ([]byte, error) {")
	g.P("    if err := proto.Unmarshal(data, req); err != nil {")
	g.P("        return nil, err")
	g.P("    }")
	g.P("    resp, err := handler(ctx, req)")
	g.P("    if err != nil {")
	g.P("        return nil, err")
	g.P("    }")
	g.P("    return proto.Marshal(resp)")
	g.P("}")
}
