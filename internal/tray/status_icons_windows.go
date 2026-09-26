package tray

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	gdi32                  = windows.NewLazySystemDLL("gdi32.dll")
	procCreateCompatibleDC = gdi32.NewProc("CreateCompatibleDC")
	procDeleteDC           = gdi32.NewProc("DeleteDC")
	procCreateDIBSection   = gdi32.NewProc("CreateDIBSection")
	procCreateBitmap       = gdi32.NewProc("CreateBitmap")
	procSelectObject       = gdi32.NewProc("SelectObject")
	procDeleteObject       = gdi32.NewProc("DeleteObject")
	procDrawIconEx         = user32.NewProc("DrawIconEx")
	procCreateIconIndirect = user32.NewProc("CreateIconIndirect")
	procGdiFlush           = gdi32.NewProc("GdiFlush")
)

type bitmapInfo struct {
	Size                         uint32
	Width, Height                int32
	Planes, BitCount             uint16
	Compression, SizeImage       uint32
	XPelsPerMeter, YPelsPerMeter int32
	ClrUsed, ClrImportant        uint32
	Colors                       [1]uint32
}

type iconInfo struct {
	IsIcon             int32
	XHotspot, YHotspot uint32
	Mask, Color        windows.Handle
}

// Keep the app artwork and add a small opaque badge. Each HICON is cached for
// the tray lifetime; all intermediate GDI objects are released immediately.
func createStatusIcon(base windows.Handle, badge int) windows.Handle {
	const size = 32
	dc, _, _ := procCreateCompatibleDC.Call(0)
	if dc == 0 {
		return 0
	}
	defer procDeleteDC.Call(dc)
	info := bitmapInfo{Size: 40, Width: size, Height: -size, Planes: 1, BitCount: 32}
	var bits unsafe.Pointer
	bitmap, _, _ := procCreateDIBSection.Call(dc, uintptr(unsafe.Pointer(&info)), 0, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if bitmap == 0 {
		return 0
	}
	defer procDeleteObject.Call(bitmap)
	previous, _, _ := procSelectObject.Call(dc, bitmap)
	defer procSelectObject.Call(dc, previous)
	pixels := unsafe.Slice((*uint32)(bits), size*size)
	clear(pixels)
	// Enlarge the artwork inside the fixed shell icon slot by trimming its
	// transparent padding. Retain a narrow margin and the full status badge.
	var offset int32 = -3
	// #nosec G115 -- DrawIconEx takes signed Win32 coordinates through uintptr.
	if ok, _, _ := procDrawIconEx.Call(dc, uintptr(offset), uintptr(offset), uintptr(base), size+6, size+6, 0, 0, 3); ok == 0 {
		return 0
	}
	procGdiFlush.Call()
	paintBadge(pixels, badge)
	var maskBits [size * size / 8]byte
	mask, _, _ := procCreateBitmap.Call(size, size, 1, 1, uintptr(unsafe.Pointer(&maskBits[0])))
	if mask == 0 {
		return 0
	}
	defer procDeleteObject.Call(mask)
	icon := iconInfo{IsIcon: 1, Mask: windows.Handle(mask), Color: windows.Handle(bitmap)}
	handle, _, _ := procCreateIconIndirect.Call(uintptr(unsafe.Pointer(&icon)))
	return windows.Handle(handle)
}

func paintBadge(pixels []uint32, badge int) {
	colors := [4]uint32{0xff64748b, 0xfff59e0b, 0xff16a34a, 0xffdc2626}
	for y := 17; y < 32; y++ {
		for x := 17; x < 32; x++ {
			dx, dy := x-24, y-24
			distance := dx*dx + dy*dy
			if distance > 49 {
				continue
			}
			color := uint32(0xffffffff)
			if distance < 36 {
				color = colors[badge]
			}
			// Distinct symbols stay legible when color alone is not enough.
			mark := false
			switch badge {
			case 0:
				mark = y == 24 && x >= 21 && x <= 27
			case 1:
				mark = x == 24 && y >= 20 && y <= 24 || y == 24 && x >= 24 && x <= 27
			case 2:
				mark = x >= 20 && x <= 23 && y == x+2 || x >= 23 && x <= 28 && y == 48-x
			case 3:
				mark = x == 24 && (y >= 20 && y <= 24 || y == 27)
			}
			if mark {
				color = 0xffffffff
			}
			pixels[y*32+x] = color
		}
	}
}
