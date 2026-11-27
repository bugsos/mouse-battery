// Command genicon generates the Mouse Battery app logo assets:
//   - assets/icon.png (512x512)
//   - assets/icon.ico (multi-size Windows icon)
//   - rsrc_windows_amd64.syso & rsrc_windows_arm64.syso (COFF resources embedded by go build)
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
)

func main() {
	if err := os.MkdirAll("assets", 0o755); err != nil {
		fatalf("mkdir assets: %v", err)
	}

	// Render master 512x512 PNG for README and macOS AppIcon.icns generation.
	master := renderLogo(512)
	if err := writePNG(filepath.Join("assets", "icon.png"), master); err != nil {
		fatalf("write assets/icon.png: %v", err)
	}

	// Render crisp multi-resolution frames for Windows ICO and embedded PE resources.
	sizes := []int{16, 24, 32, 48, 64, 128, 256}
	frames := make([][]byte, len(sizes))
	for i, sz := range sizes {
		var buf bytes.Buffer
		if err := png.Encode(&buf, renderLogo(sz)); err != nil {
			fatalf("encode %dpx png: %v", sz, err)
		}
		frames[i] = buf.Bytes()
	}

	ico := buildICO(sizes, frames)
	if err := os.WriteFile(filepath.Join("assets", "icon.ico"), ico, 0o644); err != nil {
		fatalf("write assets/icon.ico: %v", err)
	}

	const (
		machineAMD64     = 0x8664
		machineARM64     = 0xAA64
		relAMD64Addr32NB = 0x0003
		relARM64Addr32NB = 0x0002
	)
	if err := os.WriteFile("rsrc_windows_amd64.syso", buildIconSYSO(machineAMD64, relAMD64Addr32NB, sizes, frames), 0o644); err != nil {
		fatalf("write rsrc_windows_amd64.syso: %v", err)
	}
	if err := os.WriteFile("rsrc_windows_arm64.syso", buildIconSYSO(machineARM64, relARM64Addr32NB, sizes, frames), 0o644); err != nil {
		fatalf("write rsrc_windows_arm64.syso: %v", err)
	}

	fmt.Printf("Generated assets/icon.png, assets/icon.ico (%d bytes), and Windows .syso resources.\n", len(ico))
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

func writePNG(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

// renderLogo draws a minimalist, high-contrast "wireless mouse + battery" icon
// using analytical signed distance fields with 4x4 supersampling.
func renderLogo(size int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	const ss = 4
	invSS := 1.0 / float64(ss)
	invSize := 1.0 / float64(size)
	aa := 1.15 * invSize

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			var accR, accG, accB, accA float64
			for sy := 0; sy < ss; sy++ {
				for sx := 0; sx < ss; sx++ {
					u := (float64(x) + (float64(sx)+0.5)*invSS) * invSize
					v := (float64(y) + (float64(sy)+0.5)*invSS) * invSize
					r, g, b, a := sampleLogo(u, v, aa)
					accR += r * a
					accG += g * a
					accB += b * a
					accA += a
				}
			}
			n := float64(ss * ss)
			a := accA / n
			if a <= 0 {
				continue
			}
			r := clamp01((accR / n) / a)
			g := clamp01((accG / n) / a)
			b := clamp01((accB / n) / a)
			img.SetRGBA(x, y, color.RGBA{
				R: uint8(math.Round(r * a * 255)),
				G: uint8(math.Round(g * a * 255)),
				B: uint8(math.Round(b * a * 255)),
				A: uint8(math.Round(a * 255)),
			})
		}
	}
	return img
}

func sampleLogo(u, v, aa float64) (r, g, b, a float64) {
	// Outer rounded square container (fills 92% of canvas for balanced desktop/tray margins).
	dBg := sdRoundedBox(u-0.5, v-0.5, 0.46, 0.46, 0.22)
	bgMask := smoothMask(dBg, aa)
	if bgMask <= 0 {
		return 0, 0, 0, 0
	}

	// Rich emerald gradient: bright top-left -> deep forest bottom-right.
	t := clamp01(0.45*u + 0.55*v)
	r = lerp(0.15, 0.06, t)
	g = lerp(0.75, 0.44, t)
	b = lerp(0.44, 0.24, t)

	// Subtle top inner sheen for crisp modern depth.
	sheen := clamp01(1.0 - (v-0.04)/0.45)
	r = lerp(r, 1.0, sheen*0.09)
	g = lerp(g, 1.0, sheen*0.09)
	b = lerp(b, 1.0, sheen*0.09)

	// Subtle inner border ring.
	borderMask := smoothMask(math.Abs(dBg+0.012)-0.010, aa)
	r = lerp(r, 1.0, borderMask*0.14)
	g = lerp(g, 1.0, borderMask*0.14)
	b = lerp(b, 1.0, borderMask*0.14)

	// Soft drop shadow beneath the mouse body.
	dShadow := sdRoundedBox(u-0.5, v-0.525, 0.225, 0.325, 0.215)
	shadowMask := clamp01(0.5 - dShadow/0.06)
	r *= 1.0 - shadowMask*0.20
	g *= 1.0 - shadowMask*0.20
	b *= 1.0 - shadowMask*0.20

	// Wireless mouse silhouette (clean white pill body).
	dMouse := sdRoundedBox(u-0.5, v-0.5, 0.22, 0.325, 0.21)
	mouseMask := smoothMask(dMouse, aa)
	if mouseMask > 0 {
		// Subtle cool-white gradient on mouse body.
		mt := clamp01((v - 0.18) / 0.65)
		mr := lerp(1.00, 0.93, mt)
		mg := lerp(1.00, 0.96, mt)
		mb := lerp(1.00, 0.94, mt)

		// Scroll wheel capsule near top of mouse.
		dWheel := sdRoundedBox(u-0.5, v-0.315, 0.036, 0.072, 0.036)
		wheelMask := smoothMask(dWheel, aa)
		mr = lerp(mr, 0.09, wheelMask)
		mg = lerp(mg, 0.52, wheelMask)
		mb = lerp(mb, 0.29, wheelMask)

		// Subtle horizontal button seam below scroll wheel.
		dSeam := sdRoundedBox(u-0.5, v-0.435, 0.185, 0.010, 0.010)
		seamMask := smoothMask(dSeam, aa)
		mr = lerp(mr, 0.10, seamMask*0.18)
		mg = lerp(mg, 0.50, seamMask*0.18)
		mb = lerp(mb, 0.28, seamMask*0.18)

		// Battery gauge emblem centered in lower half of the mouse.
		// Outer green battery casing + right positive terminal nub.
		dBatBody := sdRoundedBox(u-0.481, v-0.615, 0.125, 0.072, 0.032)
		dBatNub := sdRoundedBox(u-0.626, v-0.615, 0.018, 0.030, 0.014)
		dBat := math.Min(dBatBody, dBatNub)
		batMask := smoothMask(dBat, aa)
		mr = lerp(mr, 0.09, batMask)
		mg = lerp(mg, 0.54, batMask)
		mb = lerp(mb, 0.30, batMask)

		// Bright luminous charge bar inside the battery casing (showing healthy ~75% charge).
		dCharge := sdRoundedBox(u-0.463, v-0.615, 0.075, 0.038, 0.018)
		chargeMask := smoothMask(dCharge, aa)
		mr = lerp(mr, 0.35, chargeMask)
		mg = lerp(mg, 0.95, chargeMask)
		mb = lerp(mb, 0.58, chargeMask)

		r = lerp(r, mr, mouseMask)
		g = lerp(g, mg, mouseMask)
		b = lerp(b, mb, mouseMask)
	}

	return r, g, b, bgMask
}

func sdRoundedBox(px, py, bx, by, r float64) float64 {
	qx := math.Abs(px) - bx + r
	qy := math.Abs(py) - by + r
	return math.Hypot(math.Max(qx, 0), math.Max(qy, 0)) + math.Min(math.Max(qx, qy), 0) - r
}

func smoothMask(d, aa float64) float64 {
	return clamp01(0.5 - d/aa)
}

func lerp(a, b, t float64) float64 {
	return a + (b-a)*t
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func buildICO(sizes []int, frames [][]byte) []byte {
	n := len(sizes)
	out := make([]byte, 6+16*n)
	le := binary.LittleEndian
	le.PutUint16(out[2:], 1)
	le.PutUint16(out[4:], uint16(n))
	for i, sz := range sizes {
		entry := out[6+i*16:]
		if sz < 256 {
			entry[0], entry[1] = byte(sz), byte(sz)
		}
		le.PutUint16(entry[4:], 1)
		le.PutUint16(entry[6:], 32)
		le.PutUint32(entry[8:], uint32(len(frames[i])))
		le.PutUint32(entry[12:], uint32(len(out)))
		out = append(out, frames[i]...)
	}
	return out
}

// buildIconSYSO produces a minimal valid Windows COFF .syso file containing
// RT_ICON (3) entries 1..N and RT_GROUP_ICON (14) entry 1.
func buildIconSYSO(machine, relocType uint16, sizes []int, frames [][]byte) []byte {
	n := len(sizes)
	le := binary.LittleEndian

	// Build RT_GROUP_ICON payload: 6-byte header + 14 bytes per icon frame.
	group := make([]byte, 6+14*n)
	le.PutUint16(group[2:], 1)
	le.PutUint16(group[4:], uint16(n))
	for i, sz := range sizes {
		e := group[6+i*14:]
		if sz < 256 {
			e[0], e[1] = byte(sz), byte(sz)
		}
		le.PutUint16(e[4:], 1)
		le.PutUint16(e[6:], 32)
		le.PutUint32(e[8:], uint32(len(frames[i])))
		le.PutUint16(e[12:], uint16(i+1)) // RT_ICON resource ID
	}

	// Layout inside .rsrc section:
	//   [root dir: 16 + 2*8 = 32]
	//   [type RT_ICON dir: 16 + n*8]
	//   [type RT_GROUP_ICON dir: 16 + 1*8 = 24]
	//   [lang dirs: (n + 1) * (16 + 8) = (n+1)*24]
	//   [data entries: (n + 1) * 16]
	//   [payloads: frames[0..n-1], group, each padded to 4 bytes]
	const (
		dirHeaderSize = 16
		dirEntrySize  = 8
		dataEntrySize = 16
		isSubdirBit   = 0x80000000
		rtIcon        = 3
		rtGroupIcon   = 14
		langENUS      = 0x0409
	)

	rootOff := 0
	typeIconOff := rootOff + dirHeaderSize + 2*dirEntrySize
	typeGroupOff := typeIconOff + dirHeaderSize + n*dirEntrySize
	langDirsOff := typeGroupOff + dirHeaderSize + 1*dirEntrySize
	dataEntriesOff := langDirsOff + (n+1)*(dirHeaderSize+dirEntrySize)
	payloadsOff := dataEntriesOff + (n+1)*dataEntrySize

	payloads := make([][]byte, n+1)
	copy(payloads, frames)
	payloads[n] = group

	payloadOffsets := make([]int, n+1)
	cur := payloadsOff
	for i, p := range payloads {
		payloadOffsets[i] = cur
		cur += (len(p) + 3) &^ 3
	}
	rsrc := make([]byte, cur)

	writeDirHeader := func(off, idCount int) {
		le.PutUint16(rsrc[off+14:], uint16(idCount))
	}
	writeDirEntry := func(off int, id, target uint32) {
		le.PutUint32(rsrc[off:], id)
		le.PutUint32(rsrc[off+4:], target)
	}

	// Root directory: RT_ICON (3) and RT_GROUP_ICON (14) in ascending ID order.
	writeDirHeader(rootOff, 2)
	writeDirEntry(rootOff+16, rtIcon, isSubdirBit|uint32(typeIconOff))
	writeDirEntry(rootOff+24, rtGroupIcon, isSubdirBit|uint32(typeGroupOff))

	// RT_ICON name directory: IDs 1..n.
	writeDirHeader(typeIconOff, n)
	for i := 0; i < n; i++ {
		langOff := langDirsOff + i*24
		writeDirEntry(typeIconOff+16+i*8, uint32(i+1), isSubdirBit|uint32(langOff))
	}

	// RT_GROUP_ICON name directory: ID 1.
	writeDirHeader(typeGroupOff, 1)
	groupLangOff := langDirsOff + n*24
	writeDirEntry(typeGroupOff+16, 1, isSubdirBit|uint32(groupLangOff))

	// Language directories, data entries, payloads, and relocation offsets.
	relocOffsets := make([]uint32, n+1)
	for i := 0; i <= n; i++ {
		langOff := langDirsOff + i*24
		dataOff := dataEntriesOff + i*dataEntrySize
		writeDirHeader(langOff, 1)
		writeDirEntry(langOff+16, langENUS, uint32(dataOff))

		le.PutUint32(rsrc[dataOff:], uint32(payloadOffsets[i]))
		le.PutUint32(rsrc[dataOff+4:], uint32(len(payloads[i])))
		relocOffsets[i] = uint32(dataOff)

		copy(rsrc[payloadOffsets[i]:], payloads[i])
	}

	// Assemble COFF object file:
	//   20-byte file header
	//   40-byte .rsrc section header
	//   len(rsrc) bytes raw data
	//   len(relocOffsets)*10 bytes relocations
	//   18-byte symbol table (1 symbol: ".rsrc")
	//   4-byte empty string table
	rawDataOff := 20 + 40
	relocsOff := rawDataOff + len(rsrc)
	symTableOff := relocsOff + len(relocOffsets)*10
	totalSize := symTableOff + 18 + 4

	coff := make([]byte, totalSize)

	// COFF file header.
	le.PutUint16(coff[0:], machine)
	le.PutUint16(coff[2:], 1) // NumberOfSections
	le.PutUint32(coff[8:], uint32(symTableOff))
	le.PutUint32(coff[12:], 1) // NumberOfSymbols

	// Section header (.rsrc).
	sec := coff[20:60]
	copy(sec[0:8], ".rsrc")
	le.PutUint32(sec[16:], uint32(len(rsrc)))
	le.PutUint32(sec[20:], uint32(rawDataOff))
	le.PutUint32(sec[24:], uint32(relocsOff))
	le.PutUint16(sec[32:], uint16(len(relocOffsets)))
	le.PutUint32(sec[36:], 0x40000040) // IMAGE_SCN_CNT_INITIALIZED_DATA | IMAGE_SCN_MEM_READ

	// Section raw data.
	copy(coff[rawDataOff:], rsrc)

	// Relocations (IMAGE_REL_*_ADDR32NB relative to symbol 0 ".rsrc").
	for i, roff := range relocOffsets {
		r := coff[relocsOff+i*10:]
		le.PutUint32(r[0:], roff)
		le.PutUint32(r[4:], 0)
		le.PutUint16(r[8:], relocType)
	}

	// Symbol 0: ".rsrc" (section 1, static).
	sym := coff[symTableOff : symTableOff+18]
	copy(sym[0:8], ".rsrc")
	le.PutUint16(sym[12:], 1) // SectionNumber = 1
	sym[16] = 3               // IMAGE_SYM_CLASS_STATIC

	// String table (empty: length = 4).
	le.PutUint32(coff[symTableOff+18:], 4)

	return coff
}
