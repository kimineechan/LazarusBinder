// Package launch coordinates independent or sequential application launches.
package launch

import (
	"errors"
	"sync"
)

var ErrCancelled = errors.New("administrator request cancelled")
var ErrAdminRequired = errors.New("this application requires administrator permission")
var ErrTrackingUnavailable = errors.New("Windows started the application without a trackable process handle")

type Request struct {
	Path       string
	RunAsAdmin bool
	Owner      uintptr
}

type Process interface {
	PID() uint32
	// Wait observes exit and releases the process resources.
	Wait() (uint32, error)
	// Release drops the wrapper's resources without stopping the application.
	Release() error
}

type State int

const (
	RequestingAdmin State = iota
	Started
	Finished
	Failed
	Cancelled
	Skipped
	WaitFailed
)

type Event struct {
	Program  int
	State    State
	PID      uint32
	ExitCode uint32
	Error    error
}

type Starter func(Request) (Process, error)

// Run returns after both applications have started, without waiting for them
// to finish. Sequential mode waits only for program 1 before starting program 2.
// It never starts program 2 in sequential mode if program 1 was cancelled,
// failed to start, or could not be observed exiting. Concurrent launches are
// independent, so one program's UAC prompt cannot hold up the other's startup.
// notify can be called from either worker and must be safe for concurrent use.
func Run(requests [2]Request, sequential bool, start Starter, notify func(Event)) {
	launchOne := func(i int) Process {
		if requests[i].RunAsAdmin {
			notify(Event{Program: i, State: RequestingAdmin})
		}
		process, err := start(requests[i])
		if err != nil {
			state := Failed
			if errors.Is(err, ErrCancelled) {
				state = Cancelled
			}
			notify(Event{Program: i, State: state, Error: err})
			return nil
		}
		notify(Event{Program: i, State: Started, PID: process.PID()})
		return process
	}
	wait := func(i int, process Process) bool {
		code, err := process.Wait()
		if err != nil {
			notify(Event{Program: i, State: WaitFailed, Error: err})
			return false
		}
		notify(Event{Program: i, State: Finished, ExitCode: code})
		return true
	}
	if sequential {
		first := launchOne(0)
		if first == nil || !wait(0, first) {
			notify(Event{Program: 1, State: Skipped})
			return
		}
		if second := launchOne(1); second != nil {
			// The wrapper is finished; the child continues independently. Any
			// remaining resources are also closed when the wrapper exits.
			_ = second.Release()
		}
		return
	}
	var workers sync.WaitGroup
	for i := range requests {
		workers.Add(1)
		go func(index int) {
			defer workers.Done()
			if process := launchOne(index); process != nil {
				_ = process.Release()
			}
		}(i)
	}
	workers.Wait()
}
