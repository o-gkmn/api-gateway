package proxy

import (
	"api-gateway/internal/logger"
	"context"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

type HealthChecker struct {
	pool      *Pool
	client    *http.Client
	path      string
	threshold int32
	interval  time.Duration
	done      chan struct{}
	stopOnce  sync.Once
	ctx       context.Context
	cancel    context.CancelFunc
}

func NewHealthChecker(
	pool *Pool,
	path string,
	threshold int32,
	interval time.Duration,
) *HealthChecker {
	hc := &HealthChecker{
		pool:      pool,
		path:      path,
		interval:  interval,
		threshold: threshold,
		done:      make(chan struct{}),
	}

	hc.client = &http.Client{
		Timeout: time.Second * 2,
		Transport: &http.Transport{
			DisableKeepAlives: true,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	hc.ctx, hc.cancel = context.WithCancel(context.Background())

	go hc.Start()

	return hc
}

func (hc *HealthChecker) Start() {
	ticker := time.NewTicker(hc.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			hc.checkAll()
		case <-hc.done:
			return
		}
	}
}

func (hc *HealthChecker) Stop() {
	hc.stopOnce.Do(func() {
		hc.cancel()
		close(hc.done)
	})
}

func (hc *HealthChecker) checkAll() {
	var wg sync.WaitGroup

	for _, inst := range hc.pool.instances {
		wg.Add(1)
		go func(inst *Instance) {
			defer wg.Done()
			hc.check(inst)
		}(inst)
	}
	wg.Wait()
}

func (hc *HealthChecker) check(instance *Instance) {
	url := instance.URL.String() + hc.path
	req, err := http.NewRequestWithContext(hc.ctx, http.MethodGet, url, nil)
	if err != nil {
		return
	}

	resp, err := hc.client.Do(req)
	if err != nil {
		hc.markFail(instance, 0, err)
		return
	}

	defer func() {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()

	if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
		hc.markHealthy(instance, resp.StatusCode)
		return
	}

	hc.markFail(instance, resp.StatusCode, nil)
}

func (hc *HealthChecker) markFail(instance *Instance, status int, err error) {
	fc := instance.consecutiveFailures.Add(1)
	if fc >= hc.threshold {
		wasHealthy := instance.Healthy.Swap(false)
		if wasHealthy {
			logger.Warn("Instance marked as unhealthy",
				slog.String("instance", instance.URL.String()),
				slog.Int("status_code", status),
				slog.Any("error", err),
			)
		}
	}
}

func (hc *HealthChecker) markHealthy(instance *Instance, status int) {
	instance.consecutiveFailures.Swap(0)
	wasHealthy := instance.Healthy.Swap(true)
	if !wasHealthy {
		logger.Info("Instance marked as healthy",
			slog.String("instance", instance.URL.String()),
			slog.Int("status_code", status),
		)
	}
}
