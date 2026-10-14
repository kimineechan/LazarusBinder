package launch

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type fakeProcess struct {
	pid     uint32
	wait    func() (uint32, error)
	release func() error
}

func (p *fakeProcess) PID() uint32           { return p.pid }
func (p *fakeProcess) Wait() (uint32, error) { return p.wait() }
func (p *fakeProcess) Release() error {
	if p.release != nil {
		return p.release()
	}
	return nil
}

func eventLog() (func(Event), func() []Event) {
	var lock sync.Mutex
	var events []Event
	return func(e Event) { lock.Lock(); defer lock.Unlock(); events = append(events, e) }, func() []Event { lock.Lock(); defer lock.Unlock(); return append([]Event(nil), events...) }
}

func TestNormalAppStartsWhileAdminApprovalIsPending(t *testing.T) {
	approval := make(chan struct{})
	normalStarted := make(chan struct{})
	done := make(chan struct{})
	notify, events := eventLog()
	start := func(r Request) (Process, error) {
		if r.RunAsAdmin {
			<-approval
			return nil, ErrCancelled
		}
		close(normalStarted)
		return &fakeProcess{pid: 2, wait: func() (uint32, error) { return 0, nil }}, nil
	}
	go func() {
		Run([2]Request{{Path: "admin.exe", RunAsAdmin: true}, {Path: "normal.exe"}}, false, start, notify)
		close(done)
	}()
	select {
	case <-normalStarted:
	case <-time.After(2 * time.Second):
		close(approval)
		t.Fatal("normal app was blocked by the administrator request")
	}
	close(approval)
	<-done
	foundCancelled, foundStarted := false, false
	for _, e := range events() {
		if e.Program == 0 && e.State == Cancelled {
			foundCancelled = true
		}
		if e.Program == 1 && e.State == Started {
			foundStarted = true
		}
	}
	if !foundCancelled || !foundStarted {
		t.Fatal("incorrect cancellation or normal-app state")
	}
}

func TestConcurrentReturnsWhileBothAppsAreStillRunning(t *testing.T) {
	appsExit := make(chan struct{})
	done := make(chan struct{})
	var waits atomic.Int32
	var releases [2]atomic.Int32
	notify, events := eventLog()
	start := func(r Request) (Process, error) {
		index := 0
		if r.Path == "second.exe" {
			index = 1
		}
		return &fakeProcess{
			pid: uint32(index + 1),
			wait: func() (uint32, error) {
				waits.Add(1)
				<-appsExit
				return 0, nil
			},
			release: func() error { releases[index].Add(1); return nil },
		}, nil
	}
	go func() {
		Run([2]Request{{Path: "first.exe"}, {Path: "second.exe"}}, false, start, notify)
		close(done)
	}()
	select {
	case <-done:
		close(appsExit)
	case <-time.After(2 * time.Second):
		close(appsExit)
		<-done
		t.Fatal("wrapper waited for the applications to finish")
	}
	if waits.Load() != 0 || releases[0].Load() != 1 || releases[1].Load() != 1 {
		t.Fatal("wrapper must release both process handles without waiting")
	}
	log := events()
	if len(log) != 2 || log[0].State != Started || log[1].State != Started {
		t.Fatal("both applications must be started before the wrapper returns")
	}
}

func TestConcurrentStartFailureDoesNotPreventOtherAppStarting(t *testing.T) {
	notify, events := eventLog()
	var released atomic.Int32
	Run([2]Request{{Path: "broken.exe"}, {Path: "working.exe"}}, false, func(r Request) (Process, error) {
		if r.Path == "broken.exe" {
			return nil, fmt.Errorf("missing runtime")
		}
		return &fakeProcess{pid: 2, release: func() error { released.Add(1); return nil }}, nil
	}, notify)
	if released.Load() != 1 {
		t.Fatal("working application was not started and released")
	}
	foundFailure, foundStarted := false, false
	for _, event := range events() {
		if event.Program == 0 && event.State == Failed {
			foundFailure = true
		}
		if event.Program == 1 && event.State == Started {
			foundStarted = true
		}
	}
	if !foundFailure || !foundStarted {
		t.Fatal("incorrect independent startup outcomes")
	}
}

func TestSequentialCancellationDoesNotStartSecondApp(t *testing.T) {
	count := 0
	notify, events := eventLog()
	Run([2]Request{{RunAsAdmin: true}, {}}, true, func(Request) (Process, error) { count++; return nil, fmt.Errorf("approval declined: %w", ErrCancelled) }, notify)
	if count != 1 {
		t.Fatal("second app started after cancellation")
	}
	last := events()
	if last[len(last)-1].State != Skipped {
		t.Fatal("missing skipped status")
	}
}

func TestSequentialWaitsOnlyForFirstAndReleasesSecond(t *testing.T) {
	firstExited := false
	count := 0
	firstReleases, secondReleases := 0, 0
	notify, _ := eventLog()
	Run([2]Request{{RunAsAdmin: true}, {}}, true, func(Request) (Process, error) {
		count++
		if count == 1 {
			return &fakeProcess{
				pid:     1,
				wait:    func() (uint32, error) { firstExited = true; return 7, nil },
				release: func() error { firstReleases++; return nil },
			}, nil
		}
		if !firstExited {
			t.Fatal("second app started before the first exited")
		}
		return &fakeProcess{
			pid:     2,
			wait:    func() (uint32, error) { t.Error("wrapper waited for program 2 to exit"); return 0, nil },
			release: func() error { secondReleases++; return nil },
		}, nil
	}, notify)
	if count != 2 {
		t.Fatal("second app never started")
	}
	if firstReleases != 0 || secondReleases != 1 {
		t.Fatal("first process must be waited for and second process must be released")
	}
}

func TestSequentialDoesNotGuessWhenProcessTrackingIsUnavailable(t *testing.T) {
	count := 0
	notify, events := eventLog()
	Run([2]Request{{RunAsAdmin: true}, {}}, true, func(Request) (Process, error) {
		count++
		return &fakeProcess{wait: func() (uint32, error) { return 0, ErrTrackingUnavailable }}, nil
	}, notify)
	if count != 1 {
		t.Fatal("second app started without observing the first exit")
	}
	last := events()
	if last[len(last)-1].State != Skipped {
		t.Fatal("missing tracking-failure status")
	}
}
