package clabernetes

import (
	"errors"

	"github.com/charmbracelet/log"
	"golang.org/x/sync/errgroup"
	"k8s.io/client-go/rest"
)

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

func warnWorkersExceedBurst(operation string, workers uint, burst, itemCount int) bool {
	if workers <= uint(burst) || itemCount <= burst {
		return false
	}
	log.Warn(
		operation+" workers exceed the Kubernetes client burst; requests may be throttled",
		"workers", workers,
		"burst", burst,
	)

	return true
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
