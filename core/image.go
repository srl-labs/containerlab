package core

import (
	"context"
	"fmt"
	"sync"

	clabnodes "github.com/srl-labs/containerlab/nodes"
)

// pullResult tracks the status of an ongoing image pull operation.
type pullResult struct {
	done chan struct{}
	err  error
}

// pullImagesForNodes concurrently pulls images for all nodes, avoiding duplicate pulls for the
// same image.
func (c *CLab) pullImagesForNodes(ctx context.Context) error {
	errCh := make(chan error, len(c.Nodes))

	var wg sync.WaitGroup

	var pullMutex sync.Mutex

	ongoingPulls := make(map[string]*pullResult)

	for _, node := range c.Nodes {
		wg.Add(1)

		go c.pullNodeImages(ctx, node, &wg, errCh, &pullMutex, ongoingPulls)
	}

	// Close the error channel when all goroutines are done
	go func() {
		wg.Wait()
		close(errCh)
	}()

	// Collect all errors
	var errors []error

	for err := range errCh {
		if err != nil {
			errors = append(errors, err)
		}
	}

	// Return the first error if any occurred
	if len(errors) > 0 {
		return errors[0]
	}

	return nil
}

// pullNodeImages pulls all images for a single node, coordinating with other goroutines
// to avoid duplicate pulls of the same image.
func (c *CLab) pullNodeImages(
	ctx context.Context, node clabnodes.Node, wg *sync.WaitGroup,
	errCh chan<- error, pullMutex *sync.Mutex, ongoingPulls map[string]*pullResult,
) {
	defer wg.Done()

	select {
	case <-ctx.Done():
		errCh <- ctx.Err()
		return
	default:
	}

	// Get all images for this node
	images := node.GetImages(ctx)

	for imageKey, imageName := range images {
		if imageName == "" {
			errCh <- fmt.Errorf(
				"missing required %q attribute for node %q", imageKey, node.Config().ShortName,
			)

			return
		}

		// Create a unique key for the image and pull policy combination
		imageKey := fmt.Sprintf("%s:%s", imageName, node.Config().ImagePullPolicy)

		pullMutex.Lock()

		if existing, found := ongoingPulls[imageKey]; found {
			// Image is already being pulled, wait for it to complete
			pullMutex.Unlock()
			<-existing.done

			if existing.err != nil {
				errCh <- existing.err
				return
			}
		} else {
			// Start a new pull for this image
			result := &pullResult{
				done: make(chan struct{}),
			}

			ongoingPulls[imageKey] = result

			pullMutex.Unlock()

			// Perform the actual pull
			err := node.GetRuntime().PullImage(ctx, imageName, node.Config().ImagePullPolicy)
			result.err = err
			close(result.done)

			if err != nil {
				errCh <- err
				return
			}
		}
	}

	errCh <- nil
}
