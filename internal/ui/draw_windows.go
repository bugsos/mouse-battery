//go:build windows

package ui

import (
	"fmt"
	"image/color"
	"math"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modGdi32   = windows.NewLazySystemDLL("gdi32.dll")
	modGdiplus = windows.NewLazySystemDLL("gdiplus.dll")

	procCreateCompatibleDC = modGdi32.NewProc("CreateCompatibleDC")
	procDeleteDC           = modGdi32.NewProc("DeleteDC")
	procCreateDIBSection   = modGdi32.NewProc("CreateDIBSection")
	procSelectObject       = modGdi32.NewProc("SelectObject")
	procDeleteObject       = modGdi32.NewProc("DeleteObject")

	procGdiplusStartup                 = modGdiplus.NewProc("GdiplusStartup")
	procGdiplusShutdown                = modGdiplus.NewProc("GdiplusShutdown")
	procGdipCreateBitmapFromScan0      = modGdiplus.NewProc("GdipCreateBitmapFromScan0")
	procGdipDisposeImage               = modGdiplus.NewProc("GdipDisposeImage")
	procGdipCreateHICONFromBitmap      = modGdiplus.NewProc("GdipCreateHICONFromBitmap")
	procGdipGetImageGraphicsContext    = modGdiplus.NewProc("GdipGetImageGraphicsContext")
	procGdipDeleteGraphics             = modGdiplus.NewProc("GdipDeleteGraphics")
	procGdipGraphicsClear              = modGdiplus.NewProc("GdipGraphicsClear")
	procGdipSetSmoothingMode           = modGdiplus.NewProc("GdipSetSmoothingMode")
	procGdipSetPixelOffsetMode         = modGdiplus.NewProc("GdipSetPixelOffsetMode")
	procGdipSetTextRenderingHint       = modGdiplus.NewProc("GdipSetTextRenderingHint")
	procGdipCreateSolidFill            = modGdiplus.NewProc("GdipCreateSolidFill")
	procGdipDeleteBrush                = modGdiplus.NewProc("GdipDeleteBrush")
	procGdipCreatePath                 = modGdiplus.NewProc("GdipCreatePath")
	procGdipDeletePath                 = modGdiplus.NewProc("GdipDeletePath")
	procGdipStartPathFigure            = modGdiplus.NewProc("GdipStartPathFigure")
	procGdipClosePathFigure            = modGdiplus.NewProc("GdipClosePathFigure")
	procGdipAddPathBeziers             = modGdiplus.NewProc("GdipAddPathBeziers")
	procGdipFillPath                   = modGdiplus.NewProc("GdipFillPath")
	procGdipCreateFontFromLogfontW     = modGdiplus.NewProc("GdipCreateFontFromLogfontW")
	procGdipDeleteFont                 = modGdiplus.NewProc("GdipDeleteFont")
	procGdipStringFormatGetGenericTypo = modGdiplus.NewProc("GdipStringFormatGetGenericTypographic")
	procGdipDeleteStringFormat         = modGdiplus.NewProc("GdipDeleteStringFormat")
	procGdipSetStringFormatAlign       = modGdiplus.NewProc("GdipSetStringFormatAlign")
	procGdipSetStringFormatLineAlign   = modGdiplus.NewProc("GdipSetStringFormatLineAlign")
	procGdipSetStringFormatFlags       = modGdiplus.NewProc("GdipSetStringFormatFlags")
	procGdipMeasureString              = modGdiplus.NewProc("GdipMeasureString")
	procGdipDrawString                 = modGdiplus.NewProc("GdipDrawString")
)

const (
	biRGB = 0

	pixelFormat32bppARGB  = 0x0026200A
	pixelFormat32bppPARGB = 0x000E200B

	smoothingModeAntiAlias        = 4
	pixelOffsetModeHalf           = 4
	textRenderingHintAntiAliasFit = 3
	textRenderingHintAntiAlias    = 4
	fillModeAlternate             = 0
	stringAlignmentCenter         = 1
	stringFormatFlagsNoWrap       = 0x00001000
	stringFormatFlagsNoClip       = 0x00004000

	fwSemiBold         = 600
	fwBold             = 700
	defaultCharset     = 1
	antialiasedQuality = 4
)

type gdiplusStartupInput struct {
	GdiplusVersion           uint32
	DebugEventCallback       uintptr
	SuppressBackgroundThread int32
	SuppressExternalCodecs   int32
}

type bitmapInfoHeader struct {
	biSize          uint32
	biWidth         int32
	biHeight        int32
	biPlanes        uint16
	biBitCount      uint16
	biCompression   uint32
	biSizeImage     uint32
	biXPelsPerMeter int32
	biYPelsPerMeter int32
	biClrUsed       uint32
	biClrImportant  uint32
}

type gpPointF struct {
	X, Y float32
}

type gpRectF struct {
	X, Y, Width, Height float32
}

type logFontW struct {
	lfHeight         int32
	lfWidth          int32
	lfEscapement     int32
	lfOrientation    int32
	lfWeight         int32
	lfItalic         byte
	lfUnderline      byte
	lfStrikeOut      byte
	lfCharSet        byte
	lfOutPrecision   byte
	lfClipPrecision  byte
	lfQuality        byte
	lfPitchAndFamily byte
	lfFaceName       [32]uint16
}

func gdiplusStartup() (uintptr, error) {
	in := gdiplusStartupInput{GdiplusVersion: 1}
	var token uintptr
	st, _, _ := procGdiplusStartup.Call(
		uintptr(unsafe.Pointer(&token)),
		uintptr(unsafe.Pointer(&in)),
		0,
	)
	if st != 0 {
		return 0, fmt.Errorf("GdiplusStartup failed with status %d", st)
	}
	return token, nil
}

func gdiplusShutdown(token uintptr) {
	if token != 0 {
		procGdiplusShutdown.Call(token)
	}
}

func argb(a, r, g, b uint8) uint32 {
	return uint32(a)<<24 | uint32(r)<<16 | uint32(g)<<8 | uint32(b)
}

func argbFromColor(c color.RGBA) uint32 {
	return argb(c.A, c.R, c.G, c.B)
}

// dibSurface wraps a 32-bit top-down premultiplied BGRA DIBSection.
type dibSurface struct {
	hdc    uintptr
	hbmp   uintptr
	oldBmp uintptr
	bits   unsafe.Pointer
	width  int32
	height int32
}

func createDIBSurface(width, height int32) (*dibSurface, error) {
	screenDC, _, _ := procGetDC.Call(0)
	memDC, _, _ := procCreateCompatibleDC.Call(screenDC)
	if screenDC != 0 {
		procReleaseDC.Call(0, screenDC)
	}
	if memDC == 0 {
		return nil, fmt.Errorf("CreateCompatibleDC failed")
	}

	bmi := bitmapInfoHeader{
		biSize:        uint32(unsafe.Sizeof(bitmapInfoHeader{})),
		biWidth:       width,
		biHeight:      -height, // top-down DIB
		biPlanes:      1,
		biBitCount:    32,
		biCompression: biRGB,
	}
	var bits unsafe.Pointer
	hbmp, _, _ := procCreateDIBSection.Call(
		memDC,
		uintptr(unsafe.Pointer(&bmi)),
		0, // DIB_RGB_COLORS
		uintptr(unsafe.Pointer(&bits)),
		0,
		0,
	)
	if hbmp == 0 || bits == nil {
		procDeleteDC.Call(memDC)
		return nil, fmt.Errorf("CreateDIBSection failed")
	}
	oldBmp, _, _ := procSelectObject.Call(memDC, hbmp)
	return &dibSurface{
		hdc:    memDC,
		hbmp:   hbmp,
		oldBmp: oldBmp,
		bits:   bits,
		width:  width,
		height: height,
	}, nil
}

func (s *dibSurface) Close() {
	if s == nil {
		return
	}
	if s.hdc != 0 && s.oldBmp != 0 {
		procSelectObject.Call(s.hdc, s.oldBmp)
	}
	if s.hbmp != 0 {
		procDeleteObject.Call(s.hbmp)
	}
	if s.hdc != 0 {
		procDeleteDC.Call(s.hdc)
	}
}

func createGdipFont(hdc uintptr, family string, pxHeight float64, weight int32) (uintptr, error) {
	h := int32(math.Round(pxHeight))
	if h < 1 {
		h = 1
	}
	var lf logFontW
	lf.lfHeight = -h
	lf.lfWeight = weight
	lf.lfCharSet = defaultCharset
	lf.lfQuality = antialiasedQuality
	utf16, err := syscall.UTF16FromString(family)
	if err == nil {
		copy(lf.lfFaceName[:len(lf.lfFaceName)-1], utf16)
	}
	var font uintptr
	st, _, _ := procGdipCreateFontFromLogfontW.Call(
		hdc,
		uintptr(unsafe.Pointer(&lf)),
		uintptr(unsafe.Pointer(&font)),
	)
	if st != 0 || font == 0 {
		return 0, fmt.Errorf("GdipCreateFontFromLogfontW(%s, %d) failed: %d", family, h, st)
	}
	return font, nil
}

func createTypographicCenterFormat() (uintptr, error) {
	var format uintptr
	st, _, _ := procGdipStringFormatGetGenericTypo.Call(uintptr(unsafe.Pointer(&format)))
	if st != 0 || format == 0 {
		return 0, fmt.Errorf("GdipStringFormatGetGenericTypographic failed: %d", st)
	}
	procGdipSetStringFormatAlign.Call(format, stringAlignmentCenter)
	procGdipSetStringFormatLineAlign.Call(format, stringAlignmentCenter)
	procGdipSetStringFormatFlags.Call(format, stringFormatFlagsNoWrap|stringFormatFlagsNoClip)
	return format, nil
}

// addRoundedRect adds a clockwise rounded rectangle figure using four cubic Beziers.
// Using pointer-based Beziers avoids float register ABI differences across Windows architectures.
func addRoundedRect(path uintptr, x, y, w, h, r float32) {
	if w <= 0 || h <= 0 {
		return
	}
	maxR := w / 2
	if h/2 < maxR {
		maxR = h / 2
	}
	if r < 0 {
		r = 0
	}
	if r > maxR {
		r = maxR
	}
	const kappa = 0.55228475
	k := r * kappa

	procGdipStartPathFigure.Call(path)
	corners := [4][4]gpPointF{
		// top-right
		{{x + w - r, y}, {x + w - r + k, y}, {x + w, y + r - k}, {x + w, y + r}},
		// bottom-right
		{{x + w, y + h - r}, {x + w, y + h - r + k}, {x + w - r + k, y + h}, {x + w - r, y + h}},
		// bottom-left
		{{x + r, y + h}, {x + r - k, y + h}, {x, y + h - r + k}, {x, y + h - r}},
		// top-left
		{{x, y + r}, {x, y + r - k}, {x + r - k, y}, {x + r, y}},
	}
	for i := range corners {
		procGdipAddPathBeziers.Call(path, uintptr(unsafe.Pointer(&corners[i][0])), 4)
	}
	procGdipClosePathFigure.Call(path)
}

func fillRoundedRect(graphics uintptr, x, y, w, h, r float32, c uint32) {
	var path uintptr
	if st, _, _ := procGdipCreatePath.Call(fillModeAlternate, uintptr(unsafe.Pointer(&path))); st != 0 || path == 0 {
		return
	}
	defer procGdipDeletePath.Call(path)
	addRoundedRect(path, x, y, w, h, r)

	var brush uintptr
	if st, _, _ := procGdipCreateSolidFill.Call(uintptr(c), uintptr(unsafe.Pointer(&brush))); st != 0 || brush == 0 {
		return
	}
	defer procGdipDeleteBrush.Call(brush)
	procGdipFillPath.Call(graphics, brush, path)
}

func fillRoundedBorder(graphics uintptr, x, y, w, h, r, borderWidth float32, c uint32) {
	if borderWidth <= 0 || w <= 2*borderWidth || h <= 2*borderWidth {
		return
	}
	var path uintptr
	if st, _, _ := procGdipCreatePath.Call(fillModeAlternate, uintptr(unsafe.Pointer(&path))); st != 0 || path == 0 {
		return
	}
	defer procGdipDeletePath.Call(path)
	addRoundedRect(path, x, y, w, h, r)
	innerR := r - borderWidth
	if innerR < 0 {
		innerR = 0
	}
	addRoundedRect(path, x+borderWidth, y+borderWidth, w-2*borderWidth, h-2*borderWidth, innerR)

	var brush uintptr
	if st, _, _ := procGdipCreateSolidFill.Call(uintptr(c), uintptr(unsafe.Pointer(&brush))); st != 0 || brush == 0 {
		return
	}
	defer procGdipDeleteBrush.Call(brush)
	procGdipFillPath.Call(graphics, brush, path)
}

func measureText(graphics, font, format uintptr, text []uint16) gpRectF {
	layout := gpRectF{X: 0, Y: 0, Width: 2048, Height: 2048}
	var bounds gpRectF
	procGdipMeasureString.Call(
		graphics,
		uintptr(unsafe.Pointer(&text[0])),
		uintptr(len(text)-1),
		font,
		uintptr(unsafe.Pointer(&layout)),
		format,
		uintptr(unsafe.Pointer(&bounds)),
		0,
		0,
	)
	return bounds
}

func drawFittedText(graphics, hdc uintptr, text string, rect gpRectF, maxPxHeight float64, weight int32, c uint32, hint int32) {
	u16, err := syscall.UTF16FromString(text)
	if err != nil || len(u16) <= 1 {
		return
	}
	format, err := createTypographicCenterFormat()
	if err != nil {
		return
	}
	defer procGdipDeleteStringFormat.Call(format)

	px := maxPxHeight
	font, err := createGdipFont(hdc, "Segoe UI", px, weight)
	if err != nil {
		return
	}

	bounds := measureText(graphics, font, format, u16)
	if bounds.Width > rect.Width && bounds.Width > 0 {
		scaled := px * float64(rect.Width) / float64(bounds.Width)
		if scaled < px {
			procGdipDeleteFont.Call(font)
			px = scaled
			font, err = createGdipFont(hdc, "Segoe UI", px, weight)
			if err != nil {
				return
			}
		}
	}
	defer procGdipDeleteFont.Call(font)

	var brush uintptr
	if st, _, _ := procGdipCreateSolidFill.Call(uintptr(c), uintptr(unsafe.Pointer(&brush))); st != 0 || brush == 0 {
		return
	}
	defer procGdipDeleteBrush.Call(brush)

	procGdipSetTextRenderingHint.Call(graphics, uintptr(hint))
	procGdipDrawString.Call(
		graphics,
		uintptr(unsafe.Pointer(&u16[0])),
		uintptr(len(u16)-1),
		font,
		uintptr(unsafe.Pointer(&rect)),
		format,
		brush,
	)
}

// renderOverlayDIB renders the 56x28 DIP floating battery pill into a premultiplied 32-bit DIBSurface.
func renderOverlayDIB(vs ViewState, dpi uint32) (*dibSurface, error) {
	if dpi == 0 {
		dpi = 96
	}
	scale := float64(dpi) / 96.0
	w := int32(math.Round(56.0 * scale))
	h := int32(math.Round(28.0 * scale))
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}

	surf, err := createDIBSurface(w, h)
	if err != nil {
		return nil, err
	}

	var bmp uintptr
	st, _, _ := procGdipCreateBitmapFromScan0.Call(
		uintptr(w),
		uintptr(h),
		uintptr(w*4),
		pixelFormat32bppPARGB,
		uintptr(surf.bits),
		uintptr(unsafe.Pointer(&bmp)),
	)
	if st != 0 || bmp == 0 {
		surf.Close()
		return nil, fmt.Errorf("GdipCreateBitmapFromScan0(overlay) failed: %d", st)
	}
	defer procGdipDisposeImage.Call(bmp)

	var g uintptr
	st, _, _ = procGdipGetImageGraphicsContext.Call(bmp, uintptr(unsafe.Pointer(&g)))
	if st != 0 || g == 0 {
		surf.Close()
		return nil, fmt.Errorf("GdipGetImageGraphicsContext(overlay) failed: %d", st)
	}
	defer procGdipDeleteGraphics.Call(g)

	procGdipGraphicsClear.Call(g, 0)
	procGdipSetSmoothingMode.Call(g, smoothingModeAntiAlias)
	procGdipSetPixelOffsetMode.Call(g, pixelOffsetModeHalf)

	s := float32(scale)
	pad := 0.5 * s
	gap := 1.5 * s
	nubW := 3.0 * s
	nubH := 9.0 * s
	nubR := 1.5 * s

	bodyX := pad
	bodyY := pad
	bodyW := float32(w) - 2*pad - gap - nubW
	bodyH := float32(h) - 2*pad
	bodyR := 8.0 * s
	borderW := float32(math.Max(1.0, math.Round(scale)))

	// Translucent white fill + subtle border matching the original battery pill design.
	fillRoundedRect(g, bodyX, bodyY, bodyW, bodyH, bodyR, argb(227, 255, 255, 255))
	fillRoundedBorder(g, bodyX, bodyY, bodyW, bodyH, bodyR, borderW, argb(61, 67, 84, 77))

	// Right-side battery terminal nub.
	nubX := bodyX + bodyW + gap
	nubY := (float32(h) - nubH) / 2
	fillRoundedRect(g, nubX, nubY, nubW, nubH, nubR, argb(99, 112, 127, 119))

	// Digits / percentage centered inside the pill body.
	textInsetX := 3.0 * s
	textRect := gpRectF{
		X:      bodyX + textInsetX,
		Y:      bodyY - 0.8*s, // compensate Segoe UI top internal leading for optical cap-height centering
		Width:  bodyW - 2*textInsetX,
		Height: bodyH,
	}
	drawFittedText(g, surf.hdc, vs.BadgeText, textRect, 17.0*scale, fwSemiBold, argbFromColor(vs.Tone), textRenderingHintAntiAliasFit)

	return surf, nil
}

func drawTrayGraphics(g, hdc uintptr, vs ViewState, size int32) {
	procGdipGraphicsClear.Call(g, 0)
	procGdipSetSmoothingMode.Call(g, smoothingModeAntiAlias)
	procGdipSetPixelOffsetMode.Call(g, pixelOffsetModeHalf)

	side := float64(size)
	// Fill almost the entire tray icon square so digits stay large and legible on HiDPI taskbars.
	pillH := side
	pillY := 0.0
	radius := side * 0.24

	fillRoundedRect(g, 0, float32(pillY), float32(side), float32(pillH), float32(radius), argbFromColor(vs.Tone))

	insetX := math.Max(1.0, math.Round(side*0.10))
	if vs.TrayText == "—" {
		insetX = math.Max(2.0, math.Round(side*0.22))
	}
	textRect := gpRectF{
		X:      float32(insetX),
		Y:      float32(pillY - side*0.03),
		Width:  float32(side - 2*insetX),
		Height: float32(pillH),
	}
	maxFontPx := pillH * 0.78
	drawFittedText(g, hdc, vs.TrayText, textRect, maxFontPx, fwBold, argb(255, 255, 255, 255), textRenderingHintAntiAlias)
}

// renderTrayHICON creates a native HICON sized exactly for the current taskbar DPI.
func renderTrayHICON(vs ViewState, size int32) (uintptr, error) {
	if size < 16 {
		size = 16
	}
	screenDC, _, _ := procGetDC.Call(0)
	memDC, _, _ := procCreateCompatibleDC.Call(screenDC)
	if screenDC != 0 {
		procReleaseDC.Call(0, screenDC)
	}
	if memDC == 0 {
		return 0, fmt.Errorf("CreateCompatibleDC failed")
	}
	defer procDeleteDC.Call(memDC)

	var bmp uintptr
	st, _, _ := procGdipCreateBitmapFromScan0.Call(
		uintptr(size),
		uintptr(size),
		0,
		pixelFormat32bppARGB,
		0,
		uintptr(unsafe.Pointer(&bmp)),
	)
	if st != 0 || bmp == 0 {
		return 0, fmt.Errorf("GdipCreateBitmapFromScan0(tray) failed: %d", st)
	}
	defer procGdipDisposeImage.Call(bmp)

	var g uintptr
	st, _, _ = procGdipGetImageGraphicsContext.Call(bmp, uintptr(unsafe.Pointer(&g)))
	if st != 0 || g == 0 {
		return 0, fmt.Errorf("GdipGetImageGraphicsContext(tray) failed: %d", st)
	}
	defer procGdipDeleteGraphics.Call(g)

	drawTrayGraphics(g, memDC, vs, size)

	var hicon uintptr
	st, _, _ = procGdipCreateHICONFromBitmap.Call(bmp, uintptr(unsafe.Pointer(&hicon)))
	if st != 0 || hicon == 0 {
		return 0, fmt.Errorf("GdipCreateHICONFromBitmap failed: %d", st)
	}
	return hicon, nil
}
