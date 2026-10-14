//go:build windows

// Package winui provides a small native Win32 interface without third-party dependencies.
package winui

import (
	"fmt"
	"math"
	"runtime"
	"sync"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

var user32 = syscall.NewLazyDLL("user32.dll")
var kernel32 = syscall.NewLazyDLL("kernel32.dll")
var gdi32 = syscall.NewLazyDLL("gdi32.dll")
var comdlg32 = syscall.NewLazyDLL("comdlg32.dll")
var shell32 = syscall.NewLazyDLL("shell32.dll")

var createWindow = user32.NewProc("CreateWindowExW")
var sendMessage = user32.NewProc("SendMessageW")
var postMessage = user32.NewProc("PostMessageW")
var showWindow = user32.NewProc("ShowWindow")
var enableWindow = user32.NewProc("EnableWindow")
var setText = user32.NewProc("SetWindowTextW")
var getText = user32.NewProc("GetWindowTextW")
var getTextLength = user32.NewProc("GetWindowTextLengthW")
var loadImage = user32.NewProc("LoadImageW")
var destroyIcon = user32.NewProc("DestroyIcon")

type point struct{ X, Y int32 }
type rect struct{ Left, Top, Right, Bottom int32 }
type message struct {
	HWND           uintptr
	ID             uint32
	WParam, LParam uintptr
	Time           uint32
	Point          point
	Private        uint32
}
type windowClass struct {
	Size, Style                        uint32
	Procedure                          uintptr
	ClassExtra, WindowExtra            int32
	Instance, Icon, Cursor, Background uintptr
	MenuName, ClassName                *uint16
	SmallIcon                          uintptr
}

type Window struct {
	Handle                       uintptr
	scale                        float64
	font, titleFont, headingFont uintptr
	controls                     []uintptr
	pages                        map[int][]uintptr
	page                         int
	nextID                       uintptr
	actions                      map[uintptr]func()
	notifications                map[uintptr]func()
	jobs                         []func()
	lock                         sync.Mutex
	icons                        map[uintptr]uintptr
	callback                     uintptr
}

func wide(s string) []uint16     { return append(utf16.Encode([]rune(s)), 0) }
func ptr(s string) *uint16       { return &wide(s)[0] }
func (w *Window) px(n int) int32 { return int32(math.Round(float64(n) * w.scale)) }

func New(title string, width, height int) (*Window, error) {
	runtime.LockOSThread()
	user32.NewProc("SetProcessDPIAware").Call()
	dpi := uintptr(96)
	if p := user32.NewProc("GetDpiForSystem"); p.Find() == nil {
		dpi, _, _ = p.Call()
	}
	if dpi == 0 {
		dpi = 96
	}
	w := &Window{scale: float64(dpi) / 96, pages: map[int][]uintptr{}, actions: map[uintptr]func(){}, notifications: map[uintptr]func(){}, icons: map[uintptr]uintptr{}, nextID: 100, page: -1}
	screenWidth, _, _ := user32.NewProc("GetSystemMetrics").Call(0)
	screenHeight, _, _ := user32.NewProc("GetSystemMetrics").Call(1)
	// Keep the complete fixed layout visible even with a small screen or high scaling.
	w.scale = math.Min(w.scale, math.Min(float64(screenWidth-60)/float64(width), float64(screenHeight-100)/float64(height)))
	if w.scale < 0.7 {
		w.scale = 0.7
	}
	instance, _, _ := kernel32.NewProc("GetModuleHandleW").Call(0)
	brush, _, _ := gdi32.NewProc("CreateSolidBrush").Call(0x00fafafa)
	cursor, _, _ := user32.NewProc("LoadCursorW").Call(0, 32512)
	icon, _, _ := loadImage.Call(instance, 1, 1, 32, 32, 0)
	w.callback = syscall.NewCallback(func(hwnd uintptr, msg uint32, wp, lp uintptr) uintptr {
		switch msg {
		case 0x0111: // WM_COMMAND
			id := wp & 0xffff
			notification := (wp >> 16) & 0xffff
			if notification == 0 {
				if action := w.actions[id]; action != nil {
					action()
					return 0
				}
			}
			if notification == 1 {
				if action := w.notifications[id]; action != nil {
					action()
					return 0
				}
			}
		case 0x8001:
			w.lock.Lock()
			jobs := w.jobs
			w.jobs = nil
			w.lock.Unlock()
			for _, job := range jobs {
				job()
			}
			return 0
		case 0x0010:
			user32.NewProc("DestroyWindow").Call(hwnd)
			return 0
		case 0x0002:
			user32.NewProc("PostQuitMessage").Call(0)
			return 0
		}
		result, _, _ := user32.NewProc("DefWindowProcW").Call(hwnd, uintptr(msg), wp, lp)
		return result
	})
	className := ptr("LazarusBinderNativeWindow")
	class := windowClass{Style: 3, Procedure: w.callback, Instance: instance, Icon: icon, SmallIcon: icon, Cursor: cursor, Background: brush, ClassName: className}
	class.Size = uint32(unsafe.Sizeof(class))
	ok, _, e := user32.NewProc("RegisterClassExW").Call(uintptr(unsafe.Pointer(&class)))
	if ok == 0 {
		return nil, fmt.Errorf("could not register window: %w", e)
	}
	style := uintptr(0x00ca0000)
	r := rect{Right: w.px(width), Bottom: w.px(height)}
	user32.NewProc("AdjustWindowRectEx").Call(uintptr(unsafe.Pointer(&r)), style, 0, 0)
	sw, _, _ := user32.NewProc("GetSystemMetrics").Call(0)
	sh, _, _ := user32.NewProc("GetSystemMetrics").Call(1)
	winWidth, winHeight := r.Right-r.Left, r.Bottom-r.Top
	x, y := (int32(sw)-winWidth)/2, (int32(sh)-winHeight)/2
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	h, _, e := createWindow.Call(0, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(ptr(title))), style, uintptr(x), uintptr(y), uintptr(winWidth), uintptr(winHeight), 0, 0, instance, 0)
	if h == 0 {
		return nil, fmt.Errorf("could not create window: %w", e)
	}
	w.Handle = h
	w.font = w.makeFont(16, 400)
	w.titleFont = w.makeFont(30, 700)
	w.headingFont = w.makeFont(20, 600)
	return w, nil
}

func (w *Window) makeFont(size, weight int) uintptr {
	height := w.px(-size)
	font, _, _ := gdi32.NewProc("CreateFontW").Call(uintptr(height), 0, 0, 0, uintptr(weight), 0, 0, 0, 1, 0, 0, 5, 0, uintptr(unsafe.Pointer(ptr("Segoe UI"))))
	return font
}

func (w *Window) control(class, text string, style uintptr, x, y, width, height int, action func()) uintptr {
	id := w.nextID
	w.nextID++
	extended := uintptr(0)
	if class == "EDIT" {
		extended = 0x200
	}
	h, _, _ := createWindow.Call(extended, uintptr(unsafe.Pointer(ptr(class))), uintptr(unsafe.Pointer(ptr(text))), 0x50000000|style, uintptr(w.px(x)), uintptr(w.px(y)), uintptr(w.px(width)), uintptr(w.px(height)), w.Handle, id, 0, 0)
	sendMessage.Call(h, 0x0030, w.font, 1)
	w.controls = append(w.controls, h)
	if w.page >= 0 {
		w.pages[w.page] = append(w.pages[w.page], h)
	}
	if action != nil {
		w.actions[id] = action
	}
	return h
}

func (w *Window) Common()       { w.page = -1 }
func (w *Window) Page(page int) { w.page = page }
func (w *Window) ShowPage(page int) {
	for n, handles := range w.pages {
		for _, h := range handles {
			show := uintptr(0)
			if n == page {
				show = 5
			}
			showWindow.Call(h, show)
		}
	}
}
func (w *Window) Label(text string, x, y, width, height int) uintptr {
	return w.control("STATIC", text, 0, x, y, width, height, nil)
}
func (w *Window) Title(text string, x, y, width, height int) uintptr {
	h := w.Label(text, x, y, width, height)
	sendMessage.Call(h, 0x0030, w.titleFont, 1)
	return h
}
func (w *Window) Heading(text string, x, y, width, height int) uintptr {
	h := w.Label(text, x, y, width, height)
	sendMessage.Call(h, 0x0030, w.headingFont, 1)
	return h
}
func (w *Window) Edit(text string, x, y, width, height int, readonly bool) uintptr {
	style := uintptr(0x10080)
	if readonly {
		style |= 0x800
	}
	return w.control("EDIT", text, style, x, y, width, height, nil)
}
func (w *Window) Button(text string, x, y, width, height int, action func()) uintptr {
	return w.control("BUTTON", text, 0x10000, x, y, width, height, action)
}
func (w *Window) Checkbox(text string, x, y, width, height int) uintptr {
	return w.control("BUTTON", text, 0x10003, x, y, width, height, nil)
}
func (w *Window) Checked(h uintptr) bool { r, _, _ := sendMessage.Call(h, 0x00f0, 0, 0); return r == 1 }
func (w *Window) SetChecked(h uintptr, checked bool) {
	value := uintptr(0)
	if checked {
		value = 1
	}
	sendMessage.Call(h, 0x00f1, value, 0)
}
func (w *Window) Text(h uintptr) string {
	n, _, _ := getTextLength.Call(h)
	buffer := make([]uint16, n+1)
	getText.Call(h, uintptr(unsafe.Pointer(&buffer[0])), n+1)
	return syscall.UTF16ToString(buffer)
}
func (w *Window) SetText(h uintptr, text string) { setText.Call(h, uintptr(unsafe.Pointer(ptr(text)))) }
func (w *Window) Enable(h uintptr, enabled bool) {
	b := uintptr(0)
	if enabled {
		b = 1
	}
	enableWindow.Call(h, b)
}
func (w *Window) Combo(x, y, width, height int, onChange func()) uintptr {
	h := w.control("COMBOBOX", "", 0x10000|0x200000|0x0003, x, y, width, height, nil)
	id, _, _ := user32.NewProc("GetDlgCtrlID").Call(h)
	w.notifications[id] = onChange
	return h
}
func (w *Window) SetItems(h uintptr, items []string) {
	sendMessage.Call(h, 0x014b, 0, 0)
	for _, item := range items {
		sendMessage.Call(h, 0x0143, 0, uintptr(unsafe.Pointer(ptr(item))))
	}
	if len(items) > 0 {
		sendMessage.Call(h, 0x014e, 0, 0)
	}
}
func (w *Window) Selection(h uintptr) int {
	r, _, _ := sendMessage.Call(h, 0x0147, 0, 0)
	return int(int32(r))
}
func (w *Window) IconControl(x, y, size int) uintptr {
	return w.control("STATIC", "", 3, x, y, size, size, nil)
}
func (w *Window) Preview(h uintptr, path string) {
	icon := uintptr(0)
	if path != "" {
		icon, _, _ = loadImage.Call(0, uintptr(unsafe.Pointer(ptr(path))), 1, uintptr(w.px(64)), uintptr(w.px(64)), 0x10)
	}
	sendMessage.Call(h, 0x0172, 1, icon)
	if old := w.icons[h]; old != 0 {
		destroyIcon.Call(old)
	}
	w.icons[h] = icon
}
func (w *Window) Alert(title, text string) {
	user32.NewProc("MessageBoxW").Call(w.Handle, uintptr(unsafe.Pointer(ptr(text))), uintptr(unsafe.Pointer(ptr(title))), 0x10)
}
func (w *Window) Info(title, text string) {
	user32.NewProc("MessageBoxW").Call(w.Handle, uintptr(unsafe.Pointer(ptr(text))), uintptr(unsafe.Pointer(ptr(title))), 0x40)
}
func (w *Window) Post(job func()) {
	w.lock.Lock()
	w.jobs = append(w.jobs, job)
	w.lock.Unlock()
	postMessage.Call(w.Handle, 0x8001, 0, 0)
}
func (w *Window) Run() {
	showWindow.Call(w.Handle, 5)
	user32.NewProc("UpdateWindow").Call(w.Handle)
	var msg message
	for {
		r, _, _ := user32.NewProc("GetMessageW").Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if r == 0 || int32(r) == -1 {
			break
		}
		isDialog, _, _ := user32.NewProc("IsDialogMessageW").Call(w.Handle, uintptr(unsafe.Pointer(&msg)))
		if isDialog != 0 {
			continue
		}
		user32.NewProc("TranslateMessage").Call(uintptr(unsafe.Pointer(&msg)))
		user32.NewProc("DispatchMessageW").Call(uintptr(unsafe.Pointer(&msg)))
	}
	for _, h := range w.icons {
		if h != 0 {
			destroyIcon.Call(h)
		}
	}
	for _, font := range []uintptr{w.font, w.titleFont, w.headingFont} {
		gdi32.NewProc("DeleteObject").Call(font)
	}
}

type openFileName struct {
	Size                         uint32
	Owner, Instance              uintptr
	Filter, CustomFilter         *uint16
	MaxCustomFilter, FilterIndex uint32
	File                         *uint16
	MaxFile                      uint32
	FileTitle                    *uint16
	MaxFileTitle                 uint32
	InitialDir, Title            *uint16
	Flags                        uint32
	FileOffset, FileExtension    uint16
	DefaultExtension             *uint16
	CustomData, Hook             uintptr
	TemplateName                 *uint16
	Reserved                     uintptr
	ReservedDWORD, FlagsEx       uint32
}

func (w *Window) FileDialog(title, filter, extension, suggested string, save bool) (string, error) {
	buffer := make([]uint16, 32768)
	copy(buffer, wide(suggested))
	filterData := wide(filter)
	config := openFileName{Owner: w.Handle, Filter: &filterData[0], File: &buffer[0], MaxFile: uint32(len(buffer)), Title: ptr(title), DefaultExtension: ptr(extension), FilterIndex: 1, Flags: 0x80000 | 0x800 | 0x8 | 0x4}
	config.Size = uint32(unsafe.Sizeof(config))
	proc := comdlg32.NewProc("GetOpenFileNameW")
	if save {
		proc = comdlg32.NewProc("GetSaveFileNameW")
		config.Flags |= 2
	} else {
		config.Flags |= 0x1000
	}
	ok, _, _ := proc.Call(uintptr(unsafe.Pointer(&config)))
	runtime.KeepAlive(filterData)
	if ok == 0 {
		code, _, _ := comdlg32.NewProc("CommDlgExtendedError").Call()
		if code != 0 {
			return "", fmt.Errorf("file dialog error 0x%x", code)
		}
		return "", nil
	}
	return syscall.UTF16ToString(buffer), nil
}

func (w *Window) OpenFolder(path string) {
	r, _, _ := shell32.NewProc("ShellExecuteW").Call(w.Handle, uintptr(unsafe.Pointer(ptr("open"))), uintptr(unsafe.Pointer(ptr(path))), 0, 0, 1)
	if r <= 32 {
		w.Alert("Could not open folder", fmt.Sprintf("Windows error %d", r))
	}
}

func Fatal(err error) {
	user32.NewProc("MessageBoxW").Call(0, uintptr(unsafe.Pointer(ptr(err.Error()))), uintptr(unsafe.Pointer(ptr("LazarusBinder"))), 0x10)
}
