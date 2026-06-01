//go:build windows

package src

import (
	"math"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

const (
	windowWidth      = 320
	windowHeight     = 52
	pillRound        = 22
	animInterval     = 16
	animTimerID      = 1
	transparentColor = 0x00FF00FF
)

const (
	wmPaint          = 0x000F
	wmTimer          = 0x0113
	wmDestroy        = 0x0002
	wmEraseBkgnd     = 0x0014
	swShowNoActivate = 4
	swHide           = 0
	wsExLayered      = 0x00080000
	wsExTransparent  = 0x00000020
	wsExToolWindow   = 0x00000080
	wsExTopMost      = 0x00000008
	wsPopup          = 0x80000000
	lwaColorKey      = 0x00000001
	lwaAlpha         = 0x00000002
	hwndTopMost      = uintptr(^uintptr(0))
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	procCreateWindowExW        = user32.NewProc("CreateWindowExW")
	procDefWindowProcW         = user32.NewProc("DefWindowProcW")
	procDestroyWindow          = user32.NewProc("DestroyWindow")
	procPostQuitMessage        = user32.NewProc("PostQuitMessage")
	procGetMessageW            = user32.NewProc("GetMessageW")
	procTranslateMessage       = user32.NewProc("TranslateMessage")
	procDispatchMessageW       = user32.NewProc("DispatchMessageW")
	procRegisterClassExW       = user32.NewProc("RegisterClassExW")
	procBeginPaint             = user32.NewProc("BeginPaint")
	procEndPaint               = user32.NewProc("EndPaint")
	procInvalidateRect         = user32.NewProc("InvalidateRect")
	procSetTimer               = user32.NewProc("SetTimer")
	procKillTimer              = user32.NewProc("KillTimer")
	procShowWindow             = user32.NewProc("ShowWindow")
	procSetWindowPos           = user32.NewProc("SetWindowPos")
	procGetSystemMetrics       = user32.NewProc("GetSystemMetrics")
	procPostMessageW           = user32.NewProc("PostMessageW")
	procSetLayeredWindowAttributes = user32.NewProc("SetLayeredWindowAttributes")
	procFillRect               = user32.NewProc("FillRect")
	procDrawTextW              = user32.NewProc("DrawTextW")
	procBitBlt                 = gdi32.NewProc("BitBlt")
	procCreateCompatibleDC     = gdi32.NewProc("CreateCompatibleDC")
	procDeleteDC               = gdi32.NewProc("DeleteDC")
	procCreateCompatibleBitmap = gdi32.NewProc("CreateCompatibleBitmap")
	procSelectObject           = gdi32.NewProc("SelectObject")
	procDeleteObject           = gdi32.NewProc("DeleteObject")
	procCreateFontW            = gdi32.NewProc("CreateFontW")
	procSetBkMode              = gdi32.NewProc("SetBkMode")
	procSetTextColor           = gdi32.NewProc("SetTextColor")
	procCreateSolidBrush       = gdi32.NewProc("CreateSolidBrush")
	procCreatePen              = gdi32.NewProc("CreatePen")
	procMoveToEx               = gdi32.NewProc("MoveToEx")
	procLineTo                 = gdi32.NewProc("LineTo")
	procGetStockObject         = gdi32.NewProc("GetStockObject")
	procRoundRect              = gdi32.NewProc("RoundRect")
	procGetModuleHandleW       = kernel32.NewProc("GetModuleHandleW")
	procCreateRoundRectRgn     = gdi32.NewProc("CreateRoundRectRgn")
	procFillRgn                = gdi32.NewProc("FillRgn")
	procSetPixel               = gdi32.NewProc("SetPixel")
)

type WNDCLASSEXW struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   uintptr
	Icon       uintptr
	Cursor     uintptr
	Background uintptr
	MenuName   *uint16
	ClassName  *uint16
	IconSm     uintptr
}

type POINT struct {
	X, Y int32
}

type MSG struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      POINT
}

type PAINTSTRUCT struct {
	Hdc         uintptr
	FErase      int32
	Rect        RECT
	FRestore    int32
	FIncUpdate  int32
	RGBReserved [32]byte
}

type RECT struct {
	Left, Top, Right, Bottom int32
}

var (
	guiHWnd      uintptr
	guiState     = "idle"
	guiDetail    = ""
	guiTargetVol = 0.0
	guiSmoothVol = 0.0
	guiOpacity   = 0.0
	guiTargetOp  = 0.0
	guiVisible   = false
	guiAnimOn    = false
	guiMu        sync.Mutex
	guiFont      uintptr
	guiStarted   = make(chan struct{})
	guiDone      chan struct{}
)

var windowProcPtr uintptr

func init() {
	windowProcPtr = syscall.NewCallback(windowProc)
}

func StartGUI() error {
	go runWindow()
	<-guiStarted
	return nil
}

func runWindow() {
	guiDone = make(chan struct{})

	inst, _, _ := procGetModuleHandleW.Call(0)
	if inst == 0 {
		return
	}

	className, _ := syscall.UTF16PtrFromString("DictateOverlayClass")

	wc := WNDCLASSEXW{
		Size:     uint32(unsafe.Sizeof(WNDCLASSEXW{})),
		WndProc:  windowProcPtr,
		Instance: inst,
		ClassName: className,
	}

	procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))

	windowName := syscall.StringToUTF16Ptr("Dictate")
	exStyle := wsExLayered | wsExTransparent | wsExToolWindow | wsExTopMost

	ret, _, _ := procCreateWindowExW.Call(
		uintptr(exStyle),
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(windowName)),
		wsPopup,
		0, 0, windowWidth, windowHeight,
		0, 0, inst, 0,
	)
	if ret == 0 {
		return
	}
	guiHWnd = ret

	guiFont, _, _ = procCreateFontW.Call(
		16, 0, 0, 0, 500, 0, 0, 0,
		uintptr(0), uintptr(0), uintptr(0), uintptr(0), uintptr(0),
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("Segoe UI"))),
	)

	procSetLayeredWindowAttributes.Call(guiHWnd, transparentColor, 0, uintptr(lwaColorKey))

	positionWindow()
	procShowWindow.Call(guiHWnd, swHide)

	close(guiStarted)

	var msg MSG
	for {
		ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if ret == 0 {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}

	if guiFont != 0 {
		procDeleteObject.Call(guiFont)
	}
	close(guiDone)
}

func positionWindow() {
	screenW, _, _ := procGetSystemMetrics.Call(0)
	screenH, _, _ := procGetSystemMetrics.Call(1)

	x := (int(screenW) - windowWidth) / 2
	y := int(screenH) - windowHeight - 100

	procSetWindowPos.Call(
		guiHWnd, uintptr(hwndTopMost),
		uintptr(x), uintptr(y),
		uintptr(windowWidth), uintptr(windowHeight),
		0x0010,
	)
}

func windowProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case wmEraseBkgnd:
		return 1
	case wmPaint:
		onPaint(hwnd)
		return 0
	case wmTimer:
		onTimer(hwnd)
		return 0
	case wmDestroy:
		procPostQuitMessage.Call(0)
		return 0
	}
	ret, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
	return ret
}

func postCmd(cmd uintptr, wParam, lParam uintptr) {
	guiMu.Lock()
	hwnd := guiHWnd
	guiMu.Unlock()
	if hwnd != 0 {
		procPostMessageW.Call(hwnd, cmd, wParam, lParam)
	}
}

func onPaint(hwnd uintptr) {
	var ps PAINTSTRUCT
	hdc, _, _ := procBeginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
	if hdc == 0 {
		return
	}
	defer procEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))

	guiMu.Lock()
	st := guiState
	dt := guiDetail
	vol := guiSmoothVol
	op := guiOpacity
	guiMu.Unlock()

	memDC, _, _ := procCreateCompatibleDC.Call(0)
	if memDC == 0 {
		return
	}
	defer procDeleteDC.Call(memDC)

	bmp, _, _ := procCreateCompatibleBitmap.Call(hdc, windowWidth, windowHeight)
	if bmp == 0 {
		return
	}
	defer procDeleteObject.Call(bmp)

	oldBmp, _, _ := procSelectObject.Call(memDC, bmp)
	defer procSelectObject.Call(memDC, oldBmp)

	bgBrush, _, _ := procCreateSolidBrush.Call(transparentColor)
	defer procDeleteObject.Call(bgBrush)
	fillRectAll(memDC, bgBrush)

	if op >= 0.005 {
		pillColor := uintptr(0x12141C)
		switch st {
		case "recording":
			pillColor = 0x141218
		case "transcribing":
			pillColor = 0x1A1214
		case "ready", "success", "clipboard":
			pillColor = 0x101814
		case "error":
			pillColor = 0x1A1212
		}

		pillBrush, _, _ := procCreateSolidBrush.Call(pillColor)
		defer procDeleteObject.Call(pillBrush)

		hrgn, _, _ := procCreateRoundRectRgn.Call(0, 0, windowWidth, windowHeight, pillRound*2, pillRound*2)
		if hrgn != 0 {
			procFillRgn.Call(memDC, hrgn, pillBrush)
			procDeleteObject.Call(hrgn)
		}

		borderColor := uintptr(0x3F96FA)
		switch st {
		case "recording":
			borderColor = 0x72EF44
		case "transcribing":
			borderColor = 0x9333EA
		case "ready", "success", "clipboard":
			borderColor = 0x22C55E
		case "error":
			borderColor = 0xEF4444
		}

		borderPen, _, _ := procCreatePen.Call(0, 1, borderColor)
		defer procDeleteObject.Call(borderPen)
		oldPen, _, _ := procSelectObject.Call(memDC, borderPen)

		nullBrush, _, _ := procGetStockObject.Call(5)
		oldBrush, _, _ := procSelectObject.Call(memDC, nullBrush)
		procRoundRect.Call(memDC, 1, 1, windowWidth-1, windowHeight-1, pillRound*2, pillRound*2)
		procSelectObject.Call(memDC, oldBrush)
		procSelectObject.Call(memDC, oldPen)

		procSetBkMode.Call(memDC, 1)
		procSetTextColor.Call(memDC, 0x00F3F4F6)
		if guiFont != 0 {
			procSelectObject.Call(memDC, guiFont)
		}

		iconCX := int32(26)
		iconCY := int32(windowHeight / 2)
		iconSize := int32(28)

		if st == "recording" || st == "transcribing" {
			drawWaveform(memDC, 12, iconCY-iconSize/2, iconSize, iconSize, vol, st)
			iconCX = 12 + iconSize + 10
		} else if st == "ready" || st == "success" || st == "clipboard" {
			drawCheckmark(memDC, iconCX, iconCY)
		} else if st == "error" {
			drawErrorIcon(memDC, iconCX, iconCY)
		}

		var label string
		switch st {
		case "recording":
			label = "Listening..."
		case "transcribing":
			label = "Converting speech to text..."
		case "ready", "success":
			label = "Transcription pasted!"
			if dt != "" {
				label = dt
			}
		case "clipboard":
			label = "Copied to clipboard!"
			if dt != "" {
				label = dt
			}
		case "error":
			label = "An error occurred"
			if dt != "" {
				if len(dt) > 40 {
					label = dt[:40] + "..."
				} else {
					label = dt
				}
			}
		}

		textRect := RECT{
			Left:   iconCX,
			Top:    0,
			Right:  windowWidth - 16,
			Bottom: windowHeight,
		}
		labelPtr := syscall.StringToUTF16Ptr(label)
		procDrawTextW.Call(memDC, uintptr(unsafe.Pointer(labelPtr)),
			uintptr(len(label)),
			uintptr(unsafe.Pointer(&textRect)),
			0x0001|0x0004|0x0020|0x4000)
	}

	procBitBlt.Call(
		hdc, 0, 0, windowWidth, windowHeight,
		memDC, 0, 0,
		0x00CC0020,
	)
}

func fillRectAll(hdc, brush uintptr) {
	rect := RECT{Right: windowWidth, Bottom: windowHeight}
	procFillRect.Call(hdc, uintptr(unsafe.Pointer(&rect)), brush)
}

func drawWaveform(hdc uintptr, x, y, w, h int32, vol float64, state string) {
	if vol < 0.05 {
		vol = 0.05
	}

	t := float64(time.Now().UnixMilli()) / 1000.0
	cx := x + w/2
	cy := y + h/2

	type waveDef struct {
		amp     float64
		freq    float64
		speed   float64
		r, g, b byte
	}

	var waves []waveDef
	if state == "recording" {
		e := 0.08 + 0.92*math.Min(1.0, vol)
		waves = []waveDef{
			{12 * e, 0.12, 6.0, 102, 153, 255},
			{8 * e, 0.18, -4.5, 153, 102, 255},
			{5 * e, 0.08, 8.0, 230, 77, 128},
		}
	} else {
		waves = []waveDef{
			{6 * (1.0 + 0.3*math.Sin(t*4.0)), 0.15, 10.0, 77, 179, 230},
			{4 * (1.0 + 0.2*math.Cos(t*3.0)), 0.22, -8.0, 128, 128, 230},
			{3 * (1.0 + 0.4*math.Sin(t*5.0)), 0.10, 12.0, 179, 77, 230},
		}
	}

	for _, wv := range waves {
		pen, _, _ := procCreatePen.Call(0, 1, uintptr(rgb(wv.r, wv.g, wv.b)))
		oldPen, _, _ := procSelectObject.Call(hdc, pen)
		procMoveToEx.Call(hdc, uintptr(cx), uintptr(cy), 0)

		phase := t * wv.speed
		for dx := int32(0); dx < w; dx++ {
			envelope := math.Sin(math.Pi * float64(dx) / float64(w))
			dy := int32(float64(cy) + wv.amp*envelope*math.Sin(float64(dx)*wv.freq+phase))
			procLineTo.Call(hdc, uintptr(cx-w/2+dx), uintptr(dy))
		}

		procSelectObject.Call(hdc, oldPen)
		procDeleteObject.Call(pen)
	}
}

func drawCheckmark(hdc uintptr, cx, cy int32) {
	pen, _, _ := procCreatePen.Call(0, 2, uintptr(rgb(51, 204, 102)))
	oldPen, _, _ := procSelectObject.Call(hdc, pen)

	procMoveToEx.Call(hdc, uintptr(cx-5), uintptr(cy), 0)
	procLineTo.Call(hdc, uintptr(cx-1), uintptr(cy+4))
	procLineTo.Call(hdc, uintptr(cx+5), uintptr(cy-3))

	procSelectObject.Call(hdc, oldPen)
	procDeleteObject.Call(pen)
}

func drawErrorIcon(hdc uintptr, cx, cy int32) {
	pen, _, _ := procCreatePen.Call(0, 2, uintptr(rgb(230, 51, 51)))
	oldPen, _, _ := procSelectObject.Call(hdc, pen)

	procMoveToEx.Call(hdc, uintptr(cx), uintptr(cy-5), 0)
	procLineTo.Call(hdc, uintptr(cx), uintptr(cy+1))

	procSelectObject.Call(hdc, oldPen)
	procDeleteObject.Call(pen)

	procSetPixel.Call(hdc, uintptr(cx), uintptr(cy+4), uintptr(rgb(230, 51, 51)))
}

func rgb(r, g, b byte) uint32 {
	return uint32(r) | uint32(g)<<8 | uint32(b)<<16
}

func onTimer(hwnd uintptr) {
	guiMu.Lock()
	needsRedraw := false

	if guiOpacity != guiTargetOp {
		diff := guiTargetOp - guiOpacity
		step := 0.08
		if math.Abs(diff) < step {
			guiOpacity = guiTargetOp
		} else if diff > 0 {
			guiOpacity += step
		} else {
			guiOpacity -= step
		}
		needsRedraw = true
		procSetLayeredWindowAttributes.Call(hwnd, transparentColor, uintptr(byte(guiOpacity*255)), uintptr(lwaColorKey|lwaAlpha))
	}

	if guiOpacity == 0 && guiTargetOp == 0 {
		if guiVisible {
			guiVisible = false
			procShowWindow.Call(hwnd, swHide)
		}
		if guiAnimOn {
			guiAnimOn = false
			procKillTimer.Call(hwnd, animTimerID)
		}
		guiMu.Unlock()
		return
	}

	if guiState == "recording" {
		guiSmoothVol = guiSmoothVol*0.7 + guiTargetVol*0.3
		needsRedraw = true
	}

	guiMu.Unlock()

	if needsRedraw {
		procInvalidateRect.Call(hwnd, 0, 0)
	}
}

func SetGUIState(state string, detail string) {
	guiMu.Lock()
	guiState = state
	guiDetail = detail

	switch state {
	case "recording":
		guiTargetVol = 0.0
		guiSmoothVol = 0.0
		showWindowLocked()
	case "transcribing":
		showWindowLocked()
	case "ready", "success", "clipboard":
		showWindowLocked()
		go func() {
			time.Sleep(2 * time.Second)
			guiMu.Lock()
			guiTargetOp = 0.0
			guiMu.Unlock()
			postCmd(wmTimer, 0, 0)
		}()
	case "error":
		showWindowLocked()
		go func() {
			time.Sleep(3 * time.Second)
			guiMu.Lock()
			guiTargetOp = 0.0
			guiMu.Unlock()
			postCmd(wmTimer, 0, 0)
		}()
	case "idle":
		guiTargetOp = 0.0
	}
	guiMu.Unlock()

	postCmd(wmTimer, 0, 0)
}

func showWindowLocked() {
	guiTargetOp = 0.95
	if !guiVisible {
		guiVisible = true
		procShowWindow.Call(guiHWnd, swShowNoActivate)
	}
	if !guiAnimOn {
		guiAnimOn = true
		procSetTimer.Call(guiHWnd, animTimerID, animInterval, 0)
	}
	procSetLayeredWindowAttributes.Call(guiHWnd, transparentColor, uintptr(byte(guiTargetOp*255)), uintptr(lwaColorKey|lwaAlpha))
}

func SendGUIVolume(vol float64) {
	guiMu.Lock()
	guiTargetVol = vol
	guiMu.Unlock()
}

func CloseGUI() {
	guiMu.Lock()
	if guiHWnd != 0 {
		if guiAnimOn {
			procKillTimer.Call(guiHWnd, animTimerID)
			guiAnimOn = false
		}
		procPostMessageW.Call(guiHWnd, wmDestroy, 0, 0)
	}
	guiMu.Unlock()
}
