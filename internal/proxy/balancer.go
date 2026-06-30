package proxy

import "sync/atomic"

type Balancer interface {
	Pick(instances []*Instance) *Instance
}

type RoundRobin struct {
	index atomic.Uint64
}

func (b *RoundRobin) Pick(instances []*Instance) *Instance {
	n := uint64(len(instances))
	if n == 0 {
		return nil
	}

	start := b.index.Add(1) - 1

	for tried := uint64(0); tried < n; tried++ {
		candidate := instances[(start+tried)%n]
		if candidate.Healthy.Load() {
			return candidate
		}
	}

	return nil
}
