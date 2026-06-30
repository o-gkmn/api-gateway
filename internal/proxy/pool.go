package proxy

import (
	"fmt"
	"net/url"
	"time"
)

type Pool struct {
	instances     []*Instance
	balancer      Balancer
	healthChecker *HealthChecker
}

func NewPool(urls []string, balancer Balancer, healthPath string, threshold int32, interval time.Duration) *Pool {
	if len(urls) == 0 {
		panic("pool: at least one URL required")
	}

	instances := parseInstances(urls)
	p := &Pool{
		instances: instances,
		balancer:  balancer,
	}

	p.healthChecker = NewHealthChecker(p, healthPath, threshold, interval)

	return p
}

func (p *Pool) Pick() *Instance {
	return p.balancer.Pick(p.instances)
}

func (p *Pool) Stop() {
	p.healthChecker.Stop()
}

func parseInstances(urls []string) []*Instance {
	instances := make([]*Instance, len(urls))
	for i, urlStr := range urls {
		uri, err := url.Parse(urlStr)
		if err != nil {
			panic(fmt.Errorf("pool: parse URL %q: %w", urlStr, err))
		}
		if uri.Scheme == "" || uri.Host == "" {
			panic(fmt.Errorf("pool: invalid URL %q (scheme and host required)", urlStr))
		}
		instances[i] = NewInstance(uri)
	}

	return instances
}
