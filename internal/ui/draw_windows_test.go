//go:build windows

package ui

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"
	"unsafe"

	"mouse-battery/internal/battery"
)

func TestWindowsGDIPlusRenderOverlayAndTray(t *testing.T) {
	token, err := gdiplusStartup()
	if err != nil {
		t.Fatalf("gdiplusStartup failed: %v", err)
	}
	defer gdiplusShutdown(token)

	snaps := []battery.Snapshot{
		{Status: battery.Reading},
		{Status: battery.Ready, Percent: 8},
		{Status: battery.Ready, Percent: 15},
		{Status: battery.Ready, Percent: 85},
		{Status: battery.Ready, Percent: 100},
	}

	for _, dpi := range []uint32{96, 144, 192} {
		for _, snap := range snaps {
			vs := buildViewState(snap, uiOptions{hasOverlay: true, overlayVisible: true})
			surf, err := renderOverlayDIB(vs, dpi)
			if err != nil {
				t.Fatalf("renderOverlayDIB(%q, dpi=%d) error: %v", vs.BadgeText, dpi, err)
			}
			// Verify premultiplied BGRA invariant and non-empty drawn pixels.
			total := int(surf.width * surf.height)
			pixels := unsafe.Slice((*byte)(surf.bits), total*4)
			nonZeroAlpha := 0
			coloredPixels := 0
			for i := 0; i < total; i++ {
				b, g, r, a := pixels[i*4], pixels[i*4+1], pixels[i*4+2], pixels[i*4+3]
				if a > 0 {
					nonZeroAlpha++
				}
				if b > a || g > a || r > a {
					surf.Close()
					t.Fatalf("premultiplied alpha violation at pixel %d (dpi=%d, text=%q): BGRA=(%d,%d,%d,%d)", i, dpi, vs.BadgeText, b, g, r, a)
				}
				if a > 200 && (r != g || g != b) {
					coloredPixels++
				}
			}
			surf.Close()
			if nonZeroAlpha == 0 {
				t.Fatalf("renderOverlayDIB(%q, dpi=%d) produced empty image", vs.BadgeText, dpi)
			}
			if snap.Status == battery.Ready && coloredPixels == 0 {
				t.Fatalf("renderOverlayDIB(%q, dpi=%d) produced no colored text pixels", vs.BadgeText, dpi)
			}
		}
	}

	for _, size := range []int32{16, 20, 24, 32} {
		for _, snap := range snaps {
			vs := buildViewState(snap, uiOptions{hasOverlay: true, overlayVisible: true})
			hicon, err := renderTrayHICON(vs, size)
			if err != nil {
				t.Fatalf("renderTrayHICON(%q, size=%d) error: %v", vs.TrayText, size, err)
			}
			if hicon == 0 {
				t.Fatalf("renderTrayHICON(%q, size=%d) returned 0", vs.TrayText, size)
			}
			procDestroyIcon.Call(hicon)
		}
	}

	// Optional visual preview export when MOUSE_BATTERY_PREVIEW_PNG is set.
	if outPath := os.Getenv("MOUSE_BATTERY_PREVIEW_PNG"); outPath != "" {
		exportPreviewSheet(t, snaps, outPath)
	}
}

func exportPreviewSheet(t *testing.T, snaps []battery.Snapshot, outPath string) {
	t.Helper()
	dpis := []uint32{96, 144, 192}
	halfH := 180
	trayH := 200
	img := image.NewRGBA(image.Rect(0, 0, 680, halfH*2+trayH))
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			switch {
			case y < halfH:
				img.SetRGBA(x, y, color.RGBA{32, 36, 42, 255}) // dark wallpaper
			case y < halfH*2:
				img.SetRGBA(x, y, color.RGBA{235, 240, 245, 255}) // light wallpaper
			default:
				img.SetRGBA(x, y, color.RGBA{24, 24, 28, 255}) // Windows dark taskbar
			}
		}
	}

	yOff := 14
	for _, dpi := range dpis {
		xOff := 14
		for _, snap := range snaps {
			vs := buildViewState(snap, uiOptions{hasOverlay: true, overlayVisible: true})
			surf, err := renderOverlayDIB(vs, dpi)
			if err != nil {
				t.Fatal(err)
			}
			blitPremulBGRA(img, xOff, yOff, surf, 1)
			blitPremulBGRA(img, xOff, yOff+halfH, surf, 1)
			xOff += int(surf.width) + 14
			surf.Close()
		}
		yOff += int(scaleDIP(28, dpi)) + 14
	}

	// Render tray icons at 16px, 24px, 32px (1x on left, 2x zoomed on right).
	yTray := halfH*2 + 12
	for _, size := range []int32{16, 24, 32} {
		x1 := 14
		x2 := 260
		for _, snap := range snaps {
			vs := buildViewState(snap, uiOptions{hasOverlay: true, overlayVisible: true})
			surf, err := createDIBSurface(size, size)
			if err != nil {
				t.Fatal(err)
			}
			var bmp, g uintptr
			procGdipCreateBitmapFromScan0.Call(uintptr(size), uintptr(size), uintptr(size*4), pixelFormat32bppPARGB, uintptr(surf.bits), uintptr(unsafe.Pointer(&bmp)))
			procGdipGetImageGraphicsContext.Call(bmp, uintptr(unsafe.Pointer(&g)))
			drawTrayGraphics(g, surf.hdc, vs, size)
			procGdipDeleteGraphics.Call(g)
			procGdipDisposeImage.Call(bmp)

			blitPremulBGRA(img, x1, yTray, surf, 1)
			blitPremulBGRA(img, x2, yTray, surf, 2)
			x1 += int(size) + 12
			x2 += int(size)*2 + 12
			surf.Close()
		}
		yTray += int(size)*2 + 12
	}

	f, err := os.Create(outPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

func blitPremulBGRA(dst *image.RGBA, ox, oy int, surf *dibSurface, scale int) {
	pixels := unsafe.Slice((*byte)(surf.bits), int(surf.width*surf.height)*4)
	for y := 0; y < int(surf.height); y++ {
		for x := 0; x < int(surf.width); x++ {
			idx := (y*int(surf.width) + x) * 4
			sb, sg, sr, sa := uint32(pixels[idx]), uint32(pixels[idx+1]), uint32(pixels[idx+2]), uint32(pixels[idx+3])
			for dy := 0; dy < scale; dy++ {
				for dx := 0; dx < scale; dx++ {
					px, py := ox+x*scale+dx, oy+y*scale+dy
					if !image.Pt(px, py).In(dst.Bounds()) {
						continue
					}
					bg := dst.RGBAAt(px, py)
					invA := 255 - sa
					outR := min(255, sr+(uint32(bg.R)*invA+127)/255)
					outG := min(255, sg+(uint32(bg.G)*invA+127)/255)
					outB := min(255, sb+(uint32(bg.B)*invA+127)/255)
					dst.SetRGBA(px, py, color.RGBA{uint8(outR), uint8(outG), uint8(outB), 255})
				}
			}
		}
	}
}
