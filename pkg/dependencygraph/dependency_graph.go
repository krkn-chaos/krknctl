// Package dependencygraph provides the algorithm and the data structures needed to solve the tool dependency graph
package dependencygraph

import "github.com/kendru/darwin/go/depgraph"

type ParentProvider interface {
	GetParent() *string
}

func NewGraphFromNodes(nodes map[string]ParentProvider) (*depgraph.Graph, error) {
	graph := depgraph.New()
	for k, v := range nodes {
		if parent := v.GetParent(); parent != nil {
			err := graph.DependOn(k, *parent)
			if err != nil {
				return nil, err
			}
		}
	}

	return graph, nil
}
