package modules

import (
	"context"
	"fmt"

	"github.com/pkg/errors"

	"github.com/aranw/services"
)

// ErrStopProcess is the error returned by a service as a hint to stop the server entirely.
var ErrStopProcess = errors.New("stop process")

// moduleService is a Service implementation that adds waiting for dependencies to start before starting,
// and dependant modules to stop before stopping this module service.
type moduleService struct {
	services.Service

	service services.Service
	name    string

	// startDeps, stopDeps return map of service names to services
	startDeps, stopDeps func(string) map[string]services.Service
}

// NewModuleService wraps a module service, and makes sure that dependencies are started/stopped before module service starts or stops.
// If any dependency fails to start, this service fails as well.
// On stop, errors from failed dependencies are ignored.
func NewModuleService(name string, service services.Service, startDeps, stopDeps func(string) map[string]services.Service) services.Service {
	w := &moduleService{
		name:      name,
		service:   service,
		startDeps: startDeps,
		stopDeps:  stopDeps,
	}

	w.Service = services.NewBasicService(w.start, w.run, w.stop)
	return w
}

func (w *moduleService) start(serviceContext context.Context) error {
	// wait until all startDeps are running
	startDeps := w.startDeps(w.name)
	for m, s := range startDeps {
		if s == nil {
			continue
		}

		err := s.AwaitRunning(serviceContext)
		if err != nil {
			return fmt.Errorf("failed to start %v, because it depends on module %v, which has failed: %w", w.name, m, err)
		}
	}

	// we don't want to let this service to stop until all dependant services are stopped,
	// so we use independent context here
	err := w.service.StartAsync(context.Background())
	if err != nil {
		return errors.Wrapf(err, "error starting module: %s", w.name)
	}

	return w.service.AwaitRunning(serviceContext)
}

func (w *moduleService) run(serviceContext context.Context) error {
	// wait until service stops, or context is canceled, whatever happens first.
	// We don't care about exact error here
	_ = w.service.AwaitTerminated(serviceContext)
	return w.service.FailureCase()
}

func (w *moduleService) stop(_ error) error {
	var err error
	if w.service.State() == services.Running {
		// Only wait for other modules, if underlying service is still running.
		w.waitForModulesToStop()

		err = services.StopAndAwaitTerminated(context.Background(), w.service)
	} else {
		err = w.service.FailureCase()
	}

	return err
}

func (w *moduleService) waitForModulesToStop() {
	// wait until all stopDeps have stopped
	stopDeps := w.stopDeps(w.name)
	for _, s := range stopDeps {
		if s == nil {
			continue
		}

		// Passed context isn't canceled, so we can only get error here, if service
		// fails. But we don't care *how* service stops, as long as it is done.
		_ = s.AwaitTerminated(context.Background())
	}
}
