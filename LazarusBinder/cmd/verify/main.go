// verify checks the release's real PE resources and a complete bundle round trip.
package main

import (
	"bytes"
	"fmt"
	"lazarusbinder/internal/bundle"
	"lazarusbinder/internal/icons"
	"lazarusbinder/internal/pecheck"
	"os"
	"path/filepath"
)

func main() {
	if err := check(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("Release verification passed: administrator and normal manifests, icons, both bundle modes and unchanged program bytes.")
}

func check() error {
	stubPath := "cmd/lazarusbinder/launcher.exe"
	adminStubPath := "cmd/lazarusbinder/launcher-admin.exe"
	appPath := "dist/LazarusBinder.exe"
	for _, path := range []string{stubPath, adminStubPath, appPath} {
		if e := bundle.ValidateEXE(path); e != nil {
			return fmt.Errorf("%s: %w", path, e)
		}
		groups, e := icons.Extract(path)
		if e != nil {
			return fmt.Errorf("%s: %w", path, e)
		}
		if len(groups) != 1 {
			return fmt.Errorf("expected one default icon group in %s", path)
		}
		images, e := icons.ParseICO(groups[0].Data)
		if e != nil {
			return e
		}
		if len(images) != 4 {
			return fmt.Errorf("expected four icon resolutions in %s", path)
		}
		level, e := pecheck.ExecutionLevel(path)
		if e != nil {
			return e
		}
		expected := "asInvoker"
		if path == adminStubPath {
			expected = "requireAdministrator"
		}
		if level != expected {
			return fmt.Errorf("%s has execution level %s instead of %s", path, level, expected)
		}
	}
	for _, requireAdmin := range []bool{false, true} {
		selected := stubPath
		if requireAdmin {
			selected = adminStubPath
		}
		if e := checkBundle(selected, [2]string{stubPath, appPath}, requireAdmin); e != nil {
			return e
		}
	}
	return nil
}

func checkBundle(template string, inputs [2]string, requireAdmin bool) error {
	stub, e := os.ReadFile(template)
	if e != nil {
		return e
	}
	dir, e := os.MkdirTemp("", "lazarusbinder-verify-")
	if e != nil {
		return e
	}
	defer os.RemoveAll(dir)
	output := filepath.Join(dir, "combined.exe")
	if e = bundle.Build(stub, inputs, bundle.Options{Title: "Verification", RequireAdmin: requireAdmin}, output, nil); e != nil {
		return e
	}
	a, e := bundle.Open(output)
	if e != nil {
		return e
	}
	defer a.Close()
	if a.Manifest.RequireAdmin != requireAdmin || a.Manifest.Programs[0].RunAsAdmin || a.Manifest.Programs[1].RunAsAdmin {
		return fmt.Errorf("combined administrator setting did not survive bundling")
	}
	level, e := pecheck.ExecutionLevel(output)
	if e != nil {
		return e
	}
	expected := "asInvoker"
	if requireAdmin {
		expected = "requireAdministrator"
	}
	if level != expected {
		return fmt.Errorf("generated combined EXE has an incorrect administrator manifest")
	}
	extract := filepath.Join(dir, "extracted")
	if e = os.Mkdir(extract, 0700); e != nil {
		return e
	}
	paths, e := a.Extract(extract)
	if e != nil {
		return e
	}
	for i, input := range inputs {
		original, e := os.ReadFile(input)
		if e != nil {
			return e
		}
		actual, e := os.ReadFile(paths[i])
		if e != nil {
			return e
		}
		if !bytes.Equal(original, actual) {
			return fmt.Errorf("program %d bytes changed", i+1)
		}
	}
	return nil
}
