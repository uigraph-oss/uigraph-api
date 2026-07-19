package catalog

import (
	"strings"
	"testing"
)

func TestRenderMermaid_edgeBetweenTwoServices(t *testing.T) {
	orders := &Service{ID: "aaaa", Name: "orders"}
	payments := &Service{ID: "bbbb", Name: "payments"}
	graph := DependencyGraph{
		Nodes: []DependencyGraphNode{
			{ID: orders.ID, Name: orders.Name, Service: orders},
			{ID: payments.ID, Name: payments.Name, Service: payments},
		},
		Edges: []ServiceDependencyEdge{
			{
				ServiceDependency: ServiceDependency{Name: "charge", SourceServiceID: orders.ID, ProviderServiceName: "payments", Type: "http", Criticality: "hard"},
				Consumer:          orders,
				Provider:          payments,
			},
		},
	}

	out := RenderMermaid(graph)

	if !strings.HasPrefix(out, "flowchart LR\n") {
		t.Fatalf("expected flowchart header, got:\n%s", out)
	}
	if !strings.Contains(out, "n0[\"orders\"]") {
		t.Errorf("expected orders node declaration, got:\n%s", out)
	}
	if !strings.Contains(out, "n1[\"payments\"]") {
		t.Errorf("expected payments node declaration, got:\n%s", out)
	}
	if !strings.Contains(out, "n0 -->|hard · http| n1") {
		t.Errorf("expected labelled edge orders->payments, got:\n%s", out)
	}
}

func TestRenderMermaid_ghostProviderStillEmitted(t *testing.T) {
	orders := &Service{ID: "aaaa", Name: "orders"}
	graph := DependencyGraph{
		Nodes: []DependencyGraphNode{
			{ID: orders.ID, Name: orders.Name, Service: orders},
			{ID: "ghost:stripe", Name: "stripe"},
		},
		Edges: []ServiceDependencyEdge{
			{
				ServiceDependency: ServiceDependency{Name: "pay", SourceServiceID: orders.ID, ProviderServiceName: "stripe", Criticality: "soft"},
				Consumer:          orders,
			},
		},
	}

	out := RenderMermaid(graph)

	if !strings.Contains(out, "[\"stripe\"]") {
		t.Errorf("expected ghost provider node, got:\n%s", out)
	}
	if !strings.Contains(out, "|soft|") {
		t.Errorf("expected soft criticality edge label, got:\n%s", out)
	}
}
