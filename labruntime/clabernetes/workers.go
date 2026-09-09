package clabernetes

import (
	"context"
	"errors"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/go-logr/logr/funcr"
	"golang.org/x/sync/errgroup"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

func withKubernetesClientDebugLogs(ctx context.Context) context.Context {
	verbosity := -1
	if log.GetLevel() <= log.DebugLevel {
		verbosity = 3
	}
	logger := funcr.New(func(prefix, args string) {
		log.Debug("Kubernetes client", "log", strings.TrimSpace(prefix+" "+args))
	}, funcr.Options{Verbosity: verbosity})

	return klog.NewContext(ctx, logger)
}

func (r *Runtime) kubernetesWorkers(maxWorkers uint) (workers, clientBurst int) {
	clientBurst = rest.DefaultBurst
	if r.restConfig != nil && r.restConfig.Burst > 0 {
		clientBurst = r.restConfig.Burst
	}
	workers = clientBurst
	if maxWorkers > 0 {
		workers = int(maxWorkers)
	}

	return workers, clientBurst
}

func runWithWorkers[T any](items []T, workers int, fn func(T) error) error {
	if len(items) == 0 {
		return nil
	}
	if workers <= 0 {
		workers = 1
	}

	errs := make([]error, len(items))
	var group errgroup.Group
	group.SetLimit(min(workers, len(items)))
	for idx, item := range items {
		group.Go(func() error {
			errs[idx] = fn(item)
			return nil
		})
	}

	_ = group.Wait()
	return errors.Join(errs...)
}
