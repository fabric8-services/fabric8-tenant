package cluster

import (
	"context"
	"fmt"
)

// Resolve a func to resolve a cluster
type Resolve func(ctx context.Context, target string) (*Cluster, error)

// NewResolve returns a new Cluster
func NewResolve(clusters []*Cluster) Resolve {
	return func(ctx context.Context, target string) (*Cluster, error) {
		for _, cluster := range clusters {
			if cleanURL(target) == cleanURL(cluster.APIURL) {
				return cluster, nil
			}
		}
		return nil, fmt.Errorf("unable to resolve cluster")
	}
}
