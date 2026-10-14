//go:build windows

package launch

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"unsafe"
)

var kernel32 = syscall.NewLazyDLL("kernel32.dll")
var shell32 = syscall.NewLazyDLL("shell32.dll")
var ole32 = syscall.NewLazyDLL("ole32.dll")
var advapi32 = syscall.NewLazyDLL("advapi32.dll")
var closeHandle = kernel32.NewProc("CloseHandle")

type commandProcess struct{ command *exec.Cmd }

func (p *commandProcess) PID() uint32    { return uint32(p.command.Process.Pid) }
func (p *commandProcess) Release() error { return p.command.Process.Release() }
func (p *commandProcess) Wait() (uint32, error) {
	err := p.command.Wait()
	if err != nil {
		var exited *exec.ExitError
		if !errors.As(err, &exited) {
			_ = p.Release()
			return 0, err
		}
	}
	return uint32(p.command.ProcessState.ExitCode()), nil
}

type shellProcess struct {
	handle uintptr
	pid    uint32
}

func (p *shellProcess) PID() uint32 { return p.pid }
func (p *shellProcess) Release() error {
	if p.handle == 0 {
		return nil
	}
	ok, _, err := closeHandle.Call(p.handle)
	if ok == 0 {
		return err
	}
	p.handle = 0
	return nil
}
func (p *shellProcess) Wait() (uint32, error) {
	if p.handle == 0 {
		return 0, ErrTrackingUnavailable
	}
	defer p.Release()
	result, _, err := kernel32.NewProc("WaitForSingleObject").Call(p.handle, 0xffffffff)
	if result != 0 {
		return 0, fmt.Errorf("could not wait for the application to exit: %v", err)
	}
	var code uint32
	ok, _, err := kernel32.NewProc("GetExitCodeProcess").Call(p.handle, uintptr(unsafe.Pointer(&code)))
	if ok == 0 {
		return 0, fmt.Errorf("could not read process exit status: %w", err)
	}
	return code, nil
}

// SHELLEXECUTEINFOW's pointer-sized members and natural alignment match Win64.
type shellExecuteInfo struct {
	Size, Mask                        uint32
	Owner                             uintptr
	Verb, File, Parameters, Directory *uint16
	Show                              int32
	Instance, IDList                  uintptr
	Class                             *uint16
	ClassKey                          uintptr
	HotKey                            uint32
	IconOrMonitor, Process            uintptr
}

// Start requests elevation only for explicitly marked programs, using runas.
// Windows retains control of UAC consent and administrator credentials.
func Start(request Request) (Process, error) {
	if !request.RunAsAdmin {
		command := exec.Command(request.Path)
		command.Dir = filepath.Dir(request.Path)
		if err := command.Start(); err != nil {
			if errors.Is(err, syscall.Errno(740)) {
				return nil, fmt.Errorf("%w: %v", ErrAdminRequired, err)
			}
			return nil, err
		}
		return &commandProcess{command: command}, nil
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	hresult, _, _ := ole32.NewProc("CoInitializeEx").Call(0, 0x2|0x4)
	if int32(hresult) < 0 {
		return nil, fmt.Errorf("could not prepare Windows administrator launch: HRESULT 0x%08x", uint32(hresult))
	}
	defer ole32.NewProc("CoUninitialize").Call()
	verb, err := syscall.UTF16PtrFromString("runas")
	if err != nil {
		return nil, err
	}
	file, err := syscall.UTF16PtrFromString(request.Path)
	if err != nil {
		return nil, err
	}
	directory, err := syscall.UTF16PtrFromString(filepath.Dir(request.Path))
	if err != nil {
		return nil, err
	}
	info := shellExecuteInfo{Mask: 0x40 | 0x100 | 0x400, Owner: request.Owner, Verb: verb, File: file, Directory: directory, Show: 1}
	info.Size = uint32(unsafe.Sizeof(info))
	ok, _, callErr := shell32.NewProc("ShellExecuteExW").Call(uintptr(unsafe.Pointer(&info)))
	runtime.KeepAlive(verb)
	runtime.KeepAlive(file)
	runtime.KeepAlive(directory)
	if ok == 0 {
		if errors.Is(callErr, syscall.Errno(1223)) {
			return nil, ErrCancelled
		}
		return nil, fmt.Errorf("Windows could not start the app as administrator: %w", callErr)
	}
	pid := uintptr(0)
	if info.Process != 0 {
		pid, _, _ = kernel32.NewProc("GetProcessId").Call(info.Process)
	}
	return &shellProcess{handle: info.Process, pid: uint32(pid)}, nil
}

func IsElevated() (bool, error) {
	process, _, _ := kernel32.NewProc("GetCurrentProcess").Call()
	var token uintptr
	ok, _, err := advapi32.NewProc("OpenProcessToken").Call(process, 0x8, uintptr(unsafe.Pointer(&token)))
	if ok == 0 {
		return false, err
	}
	defer closeHandle.Call(token)
	var elevation, length uint32
	ok, _, err = advapi32.NewProc("GetTokenInformation").Call(token, 20, uintptr(unsafe.Pointer(&elevation)), 4, uintptr(unsafe.Pointer(&length)))
	if ok == 0 {
		return false, err
	}
	return elevation != 0, nil
}
