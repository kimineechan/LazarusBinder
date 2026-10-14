//go:build windows

package bundle

import (
	"bytes"
	"lazarusbinder/internal/icons"
	"lazarusbinder/internal/pecheck"
	"os"
	"path/filepath"
	"testing"
)

func TestWindowsIconUpdatePreservesBundle(t *testing.T) {
	template := os.Getenv("LAZARUSBINDER_TEST_ADMIN_EXE")
	if template == "" {
		t.Skip("set LAZARUSBINDER_TEST_EXE to test icon updates on a compiled bundle")
	}
	stub, e := os.ReadFile(template)
	if e != nil {
		t.Fatal(e)
	}
	iconPath := filepath.Join("..", "..", "assets", "lazarusbinder.ico")
	paths, _ := inputs(t)
	root := t.TempDir()
	original := filepath.Join(root, "original.exe")
	updated := filepath.Join(root, "updated.exe")
	setter := func(path string) error { return icons.SetFile(path, iconPath) }
	if e = Build(stub, paths, Options{Title: "Two tools", RequireAdmin: true}, original, setter); e != nil {
		t.Fatal(e)
	}
	if e = Retouch(original, updated, setter); e != nil {
		t.Fatal(e)
	}
	a, e := Open(updated)
	if e != nil {
		t.Fatal(e)
	}
	defer a.Close()
	if !a.Manifest.RequireAdmin {
		t.Fatal("Win32 icon update changed administrator settings")
	}
	level, e := pecheck.ExecutionLevel(updated)
	if e != nil || level != "requireAdministrator" {
		t.Fatal("icon update changed the administrator manifest", e)
	}
	extracted, e := a.Extract(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	for i, path := range extracted {
		actual, e := os.ReadFile(path)
		if e != nil {
			t.Fatal(e)
		}
		expected, _ := os.ReadFile(paths[i])
		if !bytes.Equal(actual, expected) {
			t.Fatal("icon update changed a program")
		}
	}
}
