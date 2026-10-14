//go:build windows

package icons

import (
	"encoding/binary"
	"fmt"
	"runtime"
	"syscall"
	"unsafe"
)

var kernel = syscall.NewLazyDLL("kernel32.dll")
var beginResource = kernel.NewProc("BeginUpdateResourceW")
var updateResource = kernel.NewProc("UpdateResourceW")
var endResource = kernel.NewProc("EndUpdateResourceW")

// SetFile updates the first icon group of a generated launcher template.
func SetFile(path, icoPath string) error {
	images, err := ReadICO(icoPath)
	if err != nil {
		return err
	}
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	h, _, e := beginResource.Call(uintptr(unsafe.Pointer(p)), 0)
	if h == 0 {
		return fmt.Errorf("could not open icon resources: %w", e)
	}
	committed := false
	defer func() {
		if !committed {
			endResource.Call(h, 1)
		}
	}()
	group := make([]byte, 6+14*len(images))
	binary.LittleEndian.PutUint16(group[2:], 1)
	binary.LittleEndian.PutUint16(group[4:], uint16(len(images)))
	for i, image := range images {
		ok, _, e := updateResource.Call(h, 3, uintptr(i+1), 0, uintptr(unsafe.Pointer(&image.Data[0])), uintptr(len(image.Data)))
		runtime.KeepAlive(image.Data)
		if ok == 0 {
			return fmt.Errorf("could not write icon image: %w", e)
		}
		copy(group[6+i*14:6+i*14+12], image.Header[:])
		binary.LittleEndian.PutUint16(group[6+i*14+12:], uint16(i+1))
	}
	ok, _, e := updateResource.Call(h, 14, 1, 0, uintptr(unsafe.Pointer(&group[0])), uintptr(len(group)))
	runtime.KeepAlive(group)
	if ok == 0 {
		return fmt.Errorf("could not write icon group: %w", e)
	}
	ok, _, e = endResource.Call(h, 0)
	committed = true
	if ok == 0 {
		return fmt.Errorf("could not save icon resources: %w", e)
	}
	return nil
}
