//go:build windows

package icons

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestWindowsResourceUpdate(t *testing.T) {
	template := os.Getenv("LAZARUSBINDER_TEST_EXE")
	if template == "" {
		t.Skip("set LAZARUSBINDER_TEST_EXE to the compiled launcher to test Win32 resource updates")
	}
	data, e := os.ReadFile(template)
	if e != nil {
		t.Fatal(e)
	}
	dir := t.TempDir()
	exe := filepath.Join(dir, "launcher.exe")
	ico := filepath.Join(dir, "custom.ico")
	if e = os.WriteFile(exe, data, 0600); e != nil {
		t.Fatal(e)
	}
	custom := smallICO()
	if e = os.WriteFile(ico, custom, 0600); e != nil {
		t.Fatal(e)
	}
	if e = SetFile(exe, ico); e != nil {
		t.Fatal(e)
	}
	groups, e := Extract(exe)
	if e != nil {
		t.Fatal(e)
	}
	if len(groups) != 1 || !bytes.Equal(groups[0].Data, custom) {
		t.Fatal("Win32 icon update did not preserve the selected ICO")
	}
}
