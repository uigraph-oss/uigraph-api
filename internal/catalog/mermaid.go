package catalog

import (
	"fmt"
	"sort"
	"strings"
)

func RenderMermaid(graph DependencyGraph) string {
	labels := map[string]string{}
	for _, node := range graph.Nodes {
		labels[node.ID] = node.Name
	}

	consumerID := func(edge ServiceDependencyEdge) string {
		if edge.Consumer != nil {
			return edge.Consumer.ID
		}
		return edge.SourceServiceID
	}
	providerID := func(edge ServiceDependencyEdge) string {
		if edge.Provider != nil {
			return edge.Provider.ID
		}
		return "ghost:" + edge.ProviderServiceName
	}

	for _, edge := range graph.Edges {
		if _, ok := labels[consumerID(edge)]; !ok {
			labels[consumerID(edge)] = consumerID(edge)
		}
		if _, ok := labels[providerID(edge)]; !ok {
			labels[providerID(edge)] = edge.ProviderServiceName
		}
	}

	ids := make([]string, 0, len(labels))
	for id := range labels {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	mermaidID := map[string]string{}
	for i, id := range ids {
		mermaidID[id] = fmt.Sprintf("n%d", i)
	}

	var sb strings.Builder
	sb.WriteString("flowchart LR\n")
	for _, id := range ids {
		sb.WriteString(fmt.Sprintf("    %s[\"%s\"]\n", mermaidID[id], escapeMermaidLabel(labels[id])))
	}

	edges := make([]ServiceDependencyEdge, len(graph.Edges))
	copy(edges, graph.Edges)
	sort.Slice(edges, func(i, j int) bool {
		if consumerID(edges[i]) != consumerID(edges[j]) {
			return consumerID(edges[i]) < consumerID(edges[j])
		}
		if providerID(edges[i]) != providerID(edges[j]) {
			return providerID(edges[i]) < providerID(edges[j])
		}
		return edges[i].Name < edges[j].Name
	})

	for _, edge := range edges {
		label := edgeLabel(edge)
		if label == "" {
			sb.WriteString(fmt.Sprintf("    %s --> %s\n", mermaidID[consumerID(edge)], mermaidID[providerID(edge)]))
			continue
		}
		sb.WriteString(fmt.Sprintf("    %s -->|%s| %s\n", mermaidID[consumerID(edge)], escapeMermaidLabel(label), mermaidID[providerID(edge)]))
	}

	return sb.String()
}

func edgeLabel(edge ServiceDependencyEdge) string {
	parts := []string{}
	if edge.Criticality != "" {
		parts = append(parts, edge.Criticality)
	}
	if edge.Type != "" {
		parts = append(parts, edge.Type)
	}
	return strings.Join(parts, " · ")
}

func escapeMermaidLabel(label string) string {
	return strings.ReplaceAll(label, "\"", "#quot;")
}
