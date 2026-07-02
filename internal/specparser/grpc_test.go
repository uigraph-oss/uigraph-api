package specparser

import (
	"encoding/json"
	"testing"
)

const sampleProto = `
syntax = "proto3";

package helloworld;

message HelloRequest {
	string name = 1;
}

message HelloReply {
	string message = 1;
}

service Greeter {
	rpc SayHello(HelloRequest) returns (HelloReply);
	rpc SayHelloStream(HelloRequest) returns (stream HelloReply);
}
`

func TestParseGrpc_extractsOneMethodPerRPC(t *testing.T) {
	methods, err := ParseGrpc([]byte(sampleProto))
	if err != nil {
		t.Fatalf("ParseGrpc returned error: %v", err)
	}
	if len(methods) != 2 {
		t.Fatalf("expected 2 methods, got %d", len(methods))
	}
}

func TestParseGrpc_populatesIdentity(t *testing.T) {
	methods, err := ParseGrpc([]byte(sampleProto))
	if err != nil {
		t.Fatalf("ParseGrpc returned error: %v", err)
	}

	byName := map[string]ParsedGrpcMethod{}
	for _, m := range methods {
		byName[m.MethodName] = m
	}

	sayHello, ok := byName["SayHello"]
	if !ok {
		t.Fatal("SayHello method not found")
	}
	if sayHello.PackageName != "helloworld" {
		t.Fatalf("expected package helloworld, got %q", sayHello.PackageName)
	}
	if sayHello.ServiceName != "Greeter" {
		t.Fatalf("expected service Greeter, got %q", sayHello.ServiceName)
	}
	if sayHello.RequestType != "HelloRequest" {
		t.Fatalf("expected request type HelloRequest, got %q", sayHello.RequestType)
	}
	if sayHello.ResponseType != "HelloReply" {
		t.Fatalf("expected response type HelloReply, got %q", sayHello.ResponseType)
	}
	if sayHello.MethodID != "helloworld.Greeter/SayHello" {
		t.Fatalf("expected methodID helloworld.Greeter/SayHello, got %q", sayHello.MethodID)
	}
}

func TestParseGrpc_classifiesStreamingType(t *testing.T) {
	methods, err := ParseGrpc([]byte(sampleProto))
	if err != nil {
		t.Fatalf("ParseGrpc returned error: %v", err)
	}

	byName := map[string]ParsedGrpcMethod{}
	for _, m := range methods {
		byName[m.MethodName] = m
	}

	if byName["SayHello"].StreamingType != "UNARY" {
		t.Fatalf("expected UNARY, got %q", byName["SayHello"].StreamingType)
	}
	if byName["SayHelloStream"].StreamingType != "SERVER_STREAMING" {
		t.Fatalf("expected SERVER_STREAMING, got %q", byName["SayHelloStream"].StreamingType)
	}
}

func TestParseGrpc_examplesAreValidJSON(t *testing.T) {
	methods, err := ParseGrpc([]byte(sampleProto))
	if err != nil {
		t.Fatalf("ParseGrpc returned error: %v", err)
	}

	for _, m := range methods {
		if !json.Valid([]byte(m.RequestExample)) {
			t.Fatalf("method %s: RequestExample is not valid JSON: %s", m.MethodName, m.RequestExample)
		}
		if !json.Valid([]byte(m.ResponseExample)) {
			t.Fatalf("method %s: ResponseExample is not valid JSON: %s", m.MethodName, m.ResponseExample)
		}
	}
}

func TestParseGrpc_invalidProtoReturnsError(t *testing.T) {
	_, err := ParseGrpc([]byte("not a proto file {{{"))
	if err == nil {
		t.Fatal("expected an error for invalid proto")
	}
}

func TestParseGrpc_emptyInputReturnsError(t *testing.T) {
	_, err := ParseGrpcSchemas(nil)
	if err == nil {
		t.Fatal("expected an error for empty proto contents")
	}
}
