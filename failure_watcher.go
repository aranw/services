package services

import (
	"fmt"
)

// FailureWatcher waits for service failures, and passed them to the channel.
type FailureWatcher struct {
	ch chan error
}

func NewFailureWatcher() *FailureWatcher {
	return &FailureWatcher{ch: make(chan error)}
}

// Chan returns channel for this watcher. If watcher is nil, returns nil channel.
// Errors returned on the channel include failure case and service description.
func (w *FailureWatcher) Chan() <-chan error {
	if w == nil {
		return nil
	}
	return w.ch
}

func (w *FailureWatcher) WatchService(service Service) {
	service.AddListener(NewListener(nil, nil, nil, nil, func(from State, failure error) {
		w.ch <- fmt.Errorf("service %s failed: %w", DescribeService(service), failure)
	}))
}

func (w *FailureWatcher) WatchManager(manager *Manager) {
	manager.AddListener(NewManagerListener(nil, nil, func(service Service) {
		w.ch <- fmt.Errorf("service %s failed: %w", DescribeService(service), service.FailureCase())
	}))
}
