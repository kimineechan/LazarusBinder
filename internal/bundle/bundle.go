// Package bundle stores two standalone programs in a readable ZIP overlay.
package bundle

import (
	"archive/zip"
	"crypto/sha256"
	"debug/pe"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"lazarusbinder/internal/pecheck"
	"os"
	"path/filepath"
	"strings"
)

const magic = "TWINFORGE-PACK1!" // Preserve compatibility with existing bundles.
const footerSize = 24            // 8-byte archive length followed by the 16-byte marker.
const MaxProgramSize int64 = 1 << 30

var ErrNotBundle = errors.New("this file is not a LazarusBinder bundle")

type Program struct {
	Name       string `json:"name"`
	Entry      string `json:"entry"`
	Size       int64  `json:"size"`
	SHA256     string `json:"sha256"`
	RunAsAdmin bool   `json:"run_as_admin,omitempty"`
}

type Options struct {
	Title        string
	WaitForFirst bool
	RequireAdmin bool
}

type Manifest struct {
	Version      int       `json:"version"`
	Title        string    `json:"title"`
	WaitForFirst bool      `json:"wait_for_first"`
	RequireAdmin bool      `json:"require_admin,omitempty"`
	Programs     []Program `json:"programs"`
}

type Archive struct {
	File     *os.File
	ZIP      *zip.Reader
	Manifest Manifest
	Offset   int64
	Length   int64
}

func (a *Archive) Close() error { return a.File.Close() }

func ValidName(name string) bool {
	base := strings.ToUpper(strings.SplitN(name, ".", 2)[0])
	switch base {
	case "CON", "PRN", "AUX", "NUL", "COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9", "LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9":
		return false
	}
	return name != "" && len(name) <= 240 && strings.HasSuffix(strings.ToLower(name), ".exe") &&
		!strings.ContainsAny(name, "/\\:\x00<>\"|?*") && name != "." && name != ".."
}

// ValidateEXE examines headers only. It never loads or runs the selected program.
func ValidateEXE(path string) error {
	stat, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !stat.Mode().IsRegular() || stat.Size() <= 0 || stat.Size() > MaxProgramSize {
		return fmt.Errorf("select a regular EXE smaller than 1 GiB")
	}
	if !ValidName(filepath.Base(path)) {
		return fmt.Errorf("select a file with a valid .exe filename")
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	f, err := pecheck.Parse(file)
	if err != nil {
		return fmt.Errorf("not a Windows executable: %w", err)
	}
	defer f.Close()
	if f.Machine != pe.IMAGE_FILE_MACHINE_I386 && f.Machine != pe.IMAGE_FILE_MACHINE_AMD64 {
		return fmt.Errorf("this version supports x86 and x64 Windows programs")
	}
	if f.Characteristics&0x0002 == 0 || f.Characteristics&0x2000 != 0 {
		return fmt.Errorf("select an executable, rather than a DLL")
	}
	switch f.OptionalHeader.(type) {
	case *pe.OptionalHeader32, *pe.OptionalHeader64:
	default:
		return fmt.Errorf("missing executable headers")
	}
	return nil
}

func safeOutput(output string, inputs ...string) error {
	abs, err := filepath.Abs(output)
	if err != nil {
		return err
	}
	if !strings.EqualFold(filepath.Ext(abs), ".exe") {
		return fmt.Errorf("output must end in .exe")
	}
	outStat, _ := os.Stat(abs)
	for _, input := range inputs {
		other, err := filepath.Abs(input)
		if err != nil {
			return err
		}
		inStat, _ := os.Stat(other)
		if strings.EqualFold(abs, other) || (outStat != nil && inStat != nil && os.SameFile(outStat, inStat)) {
			return fmt.Errorf("save to a different filename; an input file cannot be overwritten")
		}
	}
	return nil
}

// Build writes a fresh launcher, optionally sets its icon, then appends a ZIP.
// Icon resources are updated before the overlay is written, preserving its bytes.
func Build(stub []byte, paths [2]string, options Options, output string, setIcon func(string) error) error {
	if err := safeOutput(output, paths[:]...); err != nil {
		return err
	}
	for _, path := range paths {
		if err := ValidateEXE(path); err != nil {
			return err
		}
	}
	if len(stub) < 64 || string(stub[:2]) != "MZ" {
		return fmt.Errorf("invalid launcher template")
	}
	title := options.Title
	if len(title) > 120 {
		return fmt.Errorf("bundle title must be 120 characters or fewer")
	}
	if strings.TrimSpace(title) == "" {
		title = "LazarusBinder bundle"
	}
	temp, err := os.CreateTemp(filepath.Dir(output), ".lazarusbinder-*.exe")
	if err != nil {
		return err
	}
	name := temp.Name()
	defer os.Remove(name)
	if _, err = temp.Write(stub); err != nil {
		temp.Close()
		return err
	}
	if err = temp.Close(); err != nil {
		return err
	}
	if setIcon != nil {
		if err = setIcon(name); err != nil {
			return err
		}
	}
	out, err := os.OpenFile(name, os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	start, err := out.Seek(0, io.SeekEnd)
	if err != nil {
		out.Close()
		return err
	}
	zw := zip.NewWriter(out)
	m := Manifest{Version: 1, Title: title, WaitForFirst: options.WaitForFirst, RequireAdmin: options.RequireAdmin}
	for i, path := range paths {
		entry := fmt.Sprintf("programs/%d.exe", i+1)
		writer, e := zw.CreateHeader(&zip.FileHeader{Name: entry, Method: zip.Deflate})
		if e != nil {
			out.Close()
			return e
		}
		in, e := os.Open(path)
		if e != nil {
			out.Close()
			return e
		}
		hash := sha256.New()
		size, e := io.Copy(io.MultiWriter(writer, hash), io.LimitReader(in, MaxProgramSize+1))
		in.Close()
		if e != nil {
			out.Close()
			return e
		}
		if size > MaxProgramSize {
			out.Close()
			return fmt.Errorf("input grew beyond the 1 GiB limit")
		}
		m.Programs = append(m.Programs, Program{Name: filepath.Base(path), Entry: entry, Size: size, SHA256: hex.EncodeToString(hash.Sum(nil))})
	}
	writer, err := zw.Create("manifest.json")
	if err != nil {
		out.Close()
		return err
	}
	if err = json.NewEncoder(writer).Encode(m); err != nil {
		out.Close()
		return err
	}
	if err = zw.Close(); err != nil {
		out.Close()
		return err
	}
	end, err := out.Seek(0, io.SeekEnd)
	if err != nil {
		out.Close()
		return err
	}
	if err = writeFooter(out, end-start); err != nil {
		out.Close()
		return err
	}
	if err = out.Sync(); err != nil {
		out.Close()
		return err
	}
	if err = out.Close(); err != nil {
		return err
	}
	return os.Rename(name, output)
}

func writeFooter(w io.Writer, length int64) error {
	var footer [footerSize]byte
	binary.LittleEndian.PutUint64(footer[:8], uint64(length))
	copy(footer[8:], magic)
	_, err := w.Write(footer[:])
	return err
}

func Open(path string) (*Archive, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	a, err := openFile(f)
	if err != nil {
		f.Close()
	}
	return a, err
}

func openFile(f *os.File) (*Archive, error) {
	stat, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if stat.Size() < footerSize+64 {
		return nil, ErrNotBundle
	}
	var footer [footerSize]byte
	if _, err = f.ReadAt(footer[:], stat.Size()-footerSize); err != nil {
		return nil, err
	}
	if string(footer[8:]) != magic {
		return nil, ErrNotBundle
	}
	length := binary.LittleEndian.Uint64(footer[:8])
	if length == 0 || length > uint64(stat.Size()-footerSize-64) {
		return nil, fmt.Errorf("invalid bundle length")
	}
	start := stat.Size() - footerSize - int64(length)
	zr, err := zip.NewReader(io.NewSectionReader(f, start, int64(length)), int64(length))
	if err != nil {
		return nil, fmt.Errorf("invalid bundle archive: %w", err)
	}
	if len(zr.File) != 3 {
		return nil, fmt.Errorf("expected exactly two programs and a manifest")
	}
	entries := map[string]*zip.File{}
	for _, entry := range zr.File {
		if _, exists := entries[entry.Name]; exists {
			return nil, fmt.Errorf("duplicate archive entry")
		}
		entries[entry.Name] = entry
	}
	meta := entries["manifest.json"]
	if meta == nil || meta.UncompressedSize64 > 16384 {
		return nil, fmt.Errorf("invalid bundle manifest")
	}
	reader, err := meta.Open()
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(io.LimitReader(reader, 16385))
	reader.Close()
	if err != nil || len(data) > 16384 {
		return nil, fmt.Errorf("could not read manifest")
	}
	var m Manifest
	if err = json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	if m.Version != 1 || len(m.Programs) != 2 || m.Title == "" || len(m.Title) > 120 {
		return nil, fmt.Errorf("unsupported bundle manifest")
	}
	for i, p := range m.Programs {
		if !ValidName(p.Name) || p.Entry != fmt.Sprintf("programs/%d.exe", i+1) || p.Size <= 0 || p.Size > MaxProgramSize {
			return nil, fmt.Errorf("invalid bundled program")
		}
		digest, e := hex.DecodeString(p.SHA256)
		entry := entries[p.Entry]
		if e != nil || len(digest) != sha256.Size || entry == nil || entry.UncompressedSize64 != uint64(p.Size) {
			return nil, fmt.Errorf("invalid program metadata")
		}
	}
	return &Archive{File: f, ZIP: zr, Manifest: m, Offset: start, Length: int64(length)}, nil
}

// Extract verifies both programs before returning any launch paths.
// Callers own the fresh extraction directory and remove it if extraction fails.
func (a *Archive) Extract(directory string) ([2]string, error) {
	var paths [2]string
	for i, p := range a.Manifest.Programs {
		folder := filepath.Join(directory, fmt.Sprintf("program-%d", i+1))
		if err := os.Mkdir(folder, 0700); err != nil {
			return paths, err
		}
		path := filepath.Join(folder, p.Name)
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0700)
		if err != nil {
			return paths, err
		}
		var entry *zip.File
		for _, candidate := range a.ZIP.File {
			if candidate.Name == p.Entry {
				entry = candidate
			}
		}
		reader, err := entry.Open()
		if err != nil {
			file.Close()
			return paths, err
		}
		hash := sha256.New()
		size, err := io.Copy(io.MultiWriter(file, hash), io.LimitReader(reader, p.Size+1))
		reader.Close()
		closeErr := file.Close()
		if err != nil {
			return paths, err
		}
		if closeErr != nil {
			return paths, closeErr
		}
		if size != p.Size || !strings.EqualFold(hex.EncodeToString(hash.Sum(nil)), p.SHA256) {
			return paths, fmt.Errorf("integrity check failed for %s", p.Name)
		}
		if err = ValidateEXE(path); err != nil {
			return paths, err
		}
		paths[i] = path
	}
	return paths, nil
}

// Retouch changes only a fresh copy's launcher icon and preserves the ZIP verbatim.
func Retouch(source, output string, setIcon func(string) error) error {
	if err := safeOutput(output, source); err != nil {
		return err
	}
	a, err := Open(source)
	if err != nil {
		return err
	}
	defer a.Close()
	temp, err := os.CreateTemp(filepath.Dir(output), ".lazarusbinder-icon-*.exe")
	if err != nil {
		return err
	}
	name := temp.Name()
	defer os.Remove(name)
	_, err = io.Copy(temp, io.NewSectionReader(a.File, 0, a.Offset))
	closeErr := temp.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err = setIcon(name); err != nil {
		return err
	}
	out, err := os.OpenFile(name, os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, io.NewSectionReader(a.File, a.Offset, a.Length))
	if err == nil {
		err = writeFooter(out, a.Length)
	}
	if err == nil {
		err = out.Sync()
	}
	closeErr = out.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(name, output)
}
