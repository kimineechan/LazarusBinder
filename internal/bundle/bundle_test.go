package bundle

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"debug/pe"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func tinyPE() []byte {
	data := make([]byte, 1024)
	copy(data, "MZ")
	binary.LittleEndian.PutUint32(data[60:], 128)
	copy(data[128:], "PE\x00\x00")
	h := pe.FileHeader{Machine: 0x14c, NumberOfSections: 1, SizeOfOptionalHeader: 224, Characteristics: 0x102}
	var header bytes.Buffer
	binary.Write(&header, binary.LittleEndian, h)
	copy(data[132:], header.Bytes())
	optional := pe.OptionalHeader32{Magic: 0x10b, AddressOfEntryPoint: 4096, ImageBase: 0x400000, SectionAlignment: 4096, FileAlignment: 512, SizeOfImage: 8192, SizeOfHeaders: 512, Subsystem: 3, NumberOfRvaAndSizes: 16}
	header.Reset()
	binary.Write(&header, binary.LittleEndian, optional)
	copy(data[152:], header.Bytes())
	section := data[376:416]
	copy(section, ".text")
	binary.LittleEndian.PutUint32(section[8:], 1)
	binary.LittleEndian.PutUint32(section[12:], 4096)
	binary.LittleEndian.PutUint32(section[16:], 512)
	binary.LittleEndian.PutUint32(section[20:], 512)
	binary.LittleEndian.PutUint32(section[36:], 0x60000020)
	data[512] = 0xc3
	return data
}

func inputs(t *testing.T) ([2]string, []byte) {
	t.Helper()
	root := t.TempDir()
	data := tinyPE()
	paths := [2]string{filepath.Join(root, "first.exe"), filepath.Join(root, "second.exe")}
	for _, p := range paths {
		if err := os.WriteFile(p, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	return paths, data
}

func TestBuildExtractAndRetouchPreservePrograms(t *testing.T) {
	paths, stub := inputs(t)
	out := filepath.Join(t.TempDir(), "combined.exe")
	if err := Build(stub, paths, Options{Title: "My tools", WaitForFirst: true, RequireAdmin: true}, out, nil); err != nil {
		t.Fatal(err)
	}
	a, err := Open(out)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	if a.Manifest.Title != "My tools" || !a.Manifest.WaitForFirst {
		t.Fatal("wrong launcher settings")
	}
	if !a.Manifest.RequireAdmin {
		t.Fatal("combined administrator setting was lost")
	}
	dir := t.TempDir()
	extracted, err := a.Extract(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range extracted {
		b, err := os.ReadFile(p)
		if err != nil || !bytes.Equal(b, stub) {
			t.Fatal("program bytes changed", err)
		}
	}
	retouched := filepath.Join(t.TempDir(), "retouched.exe")
	err = Retouch(out, retouched, func(path string) error {
		f, e := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0600)
		if e != nil {
			return e
		}
		defer f.Close()
		_, e = f.Write([]byte("simulated resource growth"))
		return e
	})
	if err != nil {
		t.Fatal(err)
	}
	b, err := Open(retouched)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	if !b.Manifest.RequireAdmin {
		t.Fatal("icon update changed the combined administrator setting")
	}
	if _, err = b.Extract(t.TempDir()); err != nil {
		t.Fatal("icon update damaged archive", err)
	}
}

func TestRejectNonEXEAndInputOverwrite(t *testing.T) {
	paths, stub := inputs(t)
	if err := Build(stub, paths, Options{Title: "Tools"}, paths[0], nil); err == nil {
		t.Fatal("allowed input overwrite")
	}
	bad := filepath.Join(t.TempDir(), "bad.exe")
	os.WriteFile(bad, []byte("not a PE"), 0600)
	if err := ValidateEXE(bad); err == nil {
		t.Fatal("accepted invalid executable")
	}
	if err := Retouch(paths[0], filepath.Join(t.TempDir(), "copy.exe"), func(string) error { return nil }); err == nil {
		t.Fatal("accepted non-bundle")
	}
}

func customArchive(t *testing.T, m Manifest, entries map[string][]byte, duplicate bool) string {
	t.Helper()
	var z bytes.Buffer
	w := zip.NewWriter(&z)
	meta, _ := w.Create("manifest.json")
	json.NewEncoder(meta).Encode(m)
	for name, data := range entries {
		entry, _ := w.Create(name)
		entry.Write(data)
	}
	if duplicate {
		entry, _ := w.Create("manifest.json")
		entry.Write([]byte("{}"))
	}
	w.Close()
	var out bytes.Buffer
	out.Write(tinyPE())
	out.Write(z.Bytes())
	writeFooter(&out, int64(z.Len()))
	path := filepath.Join(t.TempDir(), "test.exe")
	os.WriteFile(path, out.Bytes(), 0600)
	return path
}

func manifest(data []byte) Manifest {
	h := sha256.Sum256(data)
	return Manifest{Version: 1, Title: "Tools", Programs: []Program{{Name: "a.exe", Entry: "programs/1.exe", Size: int64(len(data)), SHA256: hex.EncodeToString(h[:])}, {Name: "b.exe", Entry: "programs/2.exe", Size: int64(len(data)), SHA256: hex.EncodeToString(h[:])}}}
}

func TestRejectTraversalAndHashMismatch(t *testing.T) {
	data := tinyPE()
	entries := map[string][]byte{"programs/1.exe": data, "programs/2.exe": data}
	m := manifest(data)
	m.Programs[0].Name = "..\\outside.exe"
	if a, e := Open(customArchive(t, m, entries, false)); e == nil {
		a.Close()
		t.Fatal("accepted path traversal")
	}
	m = manifest(data)
	m.Programs[0].SHA256 = strings.Repeat("0", 64)
	a, e := Open(customArchive(t, m, entries, false))
	if e != nil {
		t.Fatal(e)
	}
	defer a.Close()
	if _, e = a.Extract(t.TempDir()); e == nil {
		t.Fatal("accepted modified program")
	}
	if a, e := Open(customArchive(t, manifest(data), entries, true)); e == nil {
		a.Close()
		t.Fatal("accepted duplicate entries")
	}
}

func TestLegacyBundleDefaultsToNormalPermissions(t *testing.T) {
	data := tinyPE()
	path := customArchive(t, manifest(data), map[string][]byte{"programs/1.exe": data, "programs/2.exe": data}, false)
	a, e := Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer a.Close()
	for _, program := range a.Manifest.Programs {
		if program.RunAsAdmin {
			t.Fatal("old bundles must not request new administrator permissions")
		}
	}
	if a.Manifest.RequireAdmin {
		t.Fatal("legacy bundle gained an administrator requirement")
	}
}

func TestRejectInvalidFooterLength(t *testing.T) {
	paths, stub := inputs(t)
	out := filepath.Join(t.TempDir(), "combined.exe")
	if e := Build(stub, paths, Options{Title: "Tools"}, out, nil); e != nil {
		t.Fatal(e)
	}
	data, _ := os.ReadFile(out)
	binary.LittleEndian.PutUint64(data[len(data)-footerSize:], ^uint64(0))
	os.WriteFile(out, data, 0600)
	if a, e := Open(out); e == nil {
		a.Close()
		t.Fatal("accepted overflowing ZIP length")
	}
}

func TestRejectExcessivePEMetadata(t *testing.T) {
	data := tinyPE()
	binary.LittleEndian.PutUint32(data[144:], ^uint32(0)) // NumberOfSymbols.
	path := filepath.Join(t.TempDir(), "malformed.exe")
	os.WriteFile(path, data, 0600)
	if e := ValidateEXE(path); e == nil {
		t.Fatal("accepted excessive symbol count")
	}
	if ValidName("CON.exe") || ValidName("LPT1.exe") {
		t.Fatal("accepted Windows device filename")
	}
}
