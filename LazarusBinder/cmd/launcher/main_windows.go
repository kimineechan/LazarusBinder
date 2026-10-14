//go:build windows

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"lazarusbinder/internal/bundle"
	"lazarusbinder/internal/launch"
	"lazarusbinder/internal/winui"
)

func main() {
	if err := run(); err != nil {
		winui.Fatal(err)
	}
}

// Successful startup has no wrapper window. Run returns after both programs
// start, or after the first exits and the second starts in sequential mode.
func run() error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	a, err := bundle.Open(self)
	if err != nil {
		return fmt.Errorf("this EXE needs a bundle created by LazarusBinder: %w", err)
	}
	defer a.Close()
	m := a.Manifest
	if m.RequireAdmin {
		elevated, err := launch.IsElevated()
		if err != nil {
			return fmt.Errorf("could not check administrator access: %w", err)
		}
		if !elevated {
			return errors.New("this combined EXE must run as administrator; use Windows' Run as administrator option")
		}
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return fmt.Errorf("could not prepare programs: %w", err)
	}
	root := filepath.Join(cache, "LazarusBinder", "Runs")
	if err := os.MkdirAll(root, 0700); err != nil {
		return fmt.Errorf("could not prepare programs: %w", err)
	}
	created, err := os.MkdirTemp(root, "run-")
	if err != nil {
		return fmt.Errorf("could not prepare programs: %w", err)
	}
	paths, err := a.Extract(created)
	if err != nil {
		os.RemoveAll(created)
		return fmt.Errorf("could not extract and verify programs: %w", err)
	}
	// An administrator combined EXE starts both apps normally, giving them
	// the already-elevated parent's security context.
	requests := [2]launch.Request{
		{Path: paths[0], RunAsAdmin: !m.RequireAdmin && m.Programs[0].RunAsAdmin},
		{Path: paths[1], RunAsAdmin: !m.RequireAdmin && m.Programs[1].RunAsAdmin},
	}
	var lock sync.Mutex
	var failures []error
	launch.Run(requests, m.WaitForFirst, launch.Start, func(event launch.Event) {
		var failure error
		switch event.State {
		case launch.Failed:
			message := event.Error.Error() + "\nCheck that the program is standalone and its required runtime is installed."
			if errors.Is(event.Error, launch.ErrAdminRequired) {
				message = "This app needs administrator permission. Run the combined EXE as administrator, or rebuild it with 'Require administrator for the combined EXE' checked."
			}
			failure = fmt.Errorf("could not start %s: %s", m.Programs[event.Program].Name, message)
		case launch.WaitFailed:
			failure = fmt.Errorf("could not track %s finishing; program 2 was not started: %w", m.Programs[event.Program].Name, event.Error)
		default:
			return
		}
		lock.Lock()
		defer lock.Unlock()
		failures = append(failures, failure)
	})
	return errors.Join(failures...)
}
