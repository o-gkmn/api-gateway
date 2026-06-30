package proxy

import (
	"net/url"
	"sync/atomic"
)

type Instance struct {
	URL                 *url.URL
	Healthy             atomic.Bool
	consecutiveFailures atomic.Int32
}

func NewInstance(url *url.URL) *Instance {
	return &Instance{
		URL: url,
	}
}
