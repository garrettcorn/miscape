// Command mkmap turns a world map raster into a Lanternfall terrain image:
// one pixel per level-1 column, colored with the terrain legend
// (maps/<world>/legend.json).
//
// The Earth map is made from Natural Earth II (public domain,
// naturalearthdata.com), 1:50m "with shaded relief and water"
// (NE2_50M_SR_W.tif, an uncompressed equirectangular RGB TIFF):
//
//	go run ./tools/mkmap -src NE2_50M_SR_W.tif -legend maps/earth/legend.json -out maps/earth/terrain.png -w 3072 -h 1536
//
// Inspect source colors at places you know (to tune the classifier):
//
//	go run ./tools/mkmap -src NE2_50M_SR_W.tif -probe "53.48,-2.24 25,30"
//
// After generating, the image can be edited by hand in any paint program
// using the legend colors.
package main

import (
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"log"
	"math"
	"os"
	"strconv"
	"strings"

	"lanternfall/internal/world"
)

func main() {
	src := flag.String("src", "", "source raster: uncompressed RGB TIFF, equirectangular, whole world")
	legendPath := flag.String("legend", "maps/earth/legend.json", "legend JSON")
	out := flag.String("out", "maps/earth/terrain.png", "output PNG")
	w := flag.Int("w", 3072, "output width")
	h := flag.Int("h", 1536, "output height")
	probe := flag.String("probe", "", "print source colors at space-separated lat,lon pairs and exit")
	flag.Parse()
	log.SetFlags(0)
	if *src == "" {
		log.Fatal("-src is required")
	}
	img, err := readTIFF(*src)
	if err != nil {
		log.Fatal(err)
	}
	if *probe != "" {
		for _, ll := range strings.Fields(*probe) {
			parts := strings.Split(ll, ",")
			lat, _ := strconv.ParseFloat(parts[0], 64)
			lon, _ := strconv.ParseFloat(parts[1], 64)
			x := int((lon + 180) / 360 * float64(img.w))
			y := int((90 - lat) / 180 * float64(img.h))
			r, g, b := img.avg(x-2, y-2, x+3, y+3)
			fmt.Printf("%7.2f,%8.2f  rgb(%3d,%3d,%3d)  -> %s\n", lat, lon, r, g, b, classify(r, g, b, lat))
		}
		return
	}
	lb, err := os.ReadFile(*legendPath)
	if err != nil {
		log.Fatal(err)
	}
	legend, err := world.ParseLegend(lb)
	if err != nil {
		log.Fatal(err)
	}
	pal := color.Palette{}
	idx := map[world.Terrain]uint8{}
	for t := world.Terrain(0); t < world.NumTerrains; t++ {
		hex := legend.Hex(t)
		if hex == "" {
			continue
		}
		v, _ := strconv.ParseUint(strings.TrimPrefix(hex, "#"), 16, 32)
		idx[t] = uint8(len(pal))
		pal = append(pal, color.RGBA{uint8(v >> 16), uint8(v >> 8), uint8(v), 255})
	}
	outImg := image.NewPaletted(image.Rect(0, 0, *w, *h), pal)
	counts := map[world.Terrain]int{}
	for y := 0; y < *h; y++ {
		lat := 90 - (float64(y)+0.5)/float64(*h)*180
		sy0, sy1 := y*img.h / *h, (y+1)*img.h / *h
		for x := 0; x < *w; x++ {
			sx0, sx1 := x*img.w / *w, (x+1)*img.w / *w
			r, g, b := img.avg(sx0, sy0, max(sx1, sx0+1), max(sy1, sy0+1))
			t := classify(r, g, b, lat)
			i, ok := idx[t]
			if !ok {
				log.Fatalf("legend has no color for %s", t)
			}
			outImg.SetColorIndex(x, y, i)
			counts[t]++
		}
	}
	f, err := os.Create(*out)
	if err != nil {
		log.Fatal(err)
	}
	enc := png.Encoder{CompressionLevel: png.BestCompression}
	if err := enc.Encode(f, outImg); err != nil {
		log.Fatal(err)
	}
	f.Close()
	st, _ := os.Stat(*out)
	fmt.Printf("wrote %s (%dx%d, %d KB)\n", *out, *w, *h, st.Size()/1024)
	for t := world.Terrain(0); t < world.NumTerrains; t++ {
		if counts[t] > 0 {
			fmt.Printf("  %-9s %5.1f%%\n", t, 100*float64(counts[t])/float64(*w**h))
		}
	}
}

// classify maps a Natural Earth II color (plus latitude) to a terrain type.
// NE2 colors land by environment: dark greens are forest, light greens and
// olives are grassland and savanna, tans are desert, grey-browns are tundra
// and white is ice; water is blue.
func classify(r, g, b int, lat float64) world.Terrain {
	R, G, B := float64(r), float64(g), float64(b)
	mx := math.Max(R, math.Max(G, B))
	mn := math.Min(R, math.Min(G, B))
	light := (mx + mn) / 2
	sat := 0.0
	if mx > 0 {
		sat = (mx - mn) / mx
	}
	switch {
	case B > R+12 && B >= G-4 && !(light > 215 && sat < 0.12):
		// Water: blue dominant. Pale, light water is shallow.
		if light > 170 {
			return world.TShallows
		}
		return world.TOcean
	case light > 222 && sat < 0.1 && math.Abs(lat) > 50:
		return world.TIce
	case G-B >= 15 && (R-G >= 10 || (light > 215 && R >= G-6)) && light >= 180:
		// Tans and pale yellows: Sahara, Arabia, Australia, the US southwest.
		return world.TDesert
	case G-R >= 32:
		// The deepest greens: rainforest and taiga.
		return world.TForest
	case math.Abs(lat) > 64:
		return world.TTundra
	case light > 205 && sat < 0.12:
		return world.TSnow
	case sat < 0.1 && light >= 150:
		// Grey uplands: Tibet, the Alps, the Andes.
		return world.TRock
	default:
		return world.TGrass
	}
}

type rgbImage struct {
	w, h int
	pix  []byte // RGB
}

func (m *rgbImage) avg(x0, y0, x1, y1 int) (int, int, int) {
	var r, g, b, n int
	for y := max(y0, 0); y < min(y1, m.h); y++ {
		for x := x0; x < x1; x++ {
			xx := ((x % m.w) + m.w) % m.w
			i := (y*m.w + xx) * 3
			r += int(m.pix[i])
			g += int(m.pix[i+1])
			b += int(m.pix[i+2])
			n++
		}
	}
	if n == 0 {
		return 0, 0, 0
	}
	return r / n, g / n, b / n
}

// readTIFF reads a little-endian, uncompressed, 8-bit RGB striped TIFF.
func readTIFF(path string) (*rgbImage, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	hdr := make([]byte, 8)
	if _, err := io.ReadFull(f, hdr); err != nil {
		return nil, err
	}
	if string(hdr[:4]) != "II*\x00" {
		return nil, errors.New("only little-endian classic TIFF is supported")
	}
	le := binary.LittleEndian
	if _, err := f.Seek(int64(le.Uint32(hdr[4:])), io.SeekStart); err != nil {
		return nil, err
	}
	var n uint16
	binary.Read(f, le, &n)
	type entry struct {
		Tag, Type uint16
		Count     uint32
		Value     uint32
	}
	tags := map[uint16]entry{}
	for i := 0; i < int(n); i++ {
		var e entry
		if err := binary.Read(f, le, &e); err != nil {
			return nil, err
		}
		tags[e.Tag] = e
	}
	val := func(t uint16) uint32 {
		e := tags[t]
		if e.Type == 3 { // SHORT stored in the low half
			return e.Value & 0xffff
		}
		return e.Value
	}
	w, h := int(val(256)), int(val(257))
	if val(259) != 1 {
		return nil, fmt.Errorf("compressed TIFF (compression %d) is not supported", val(259))
	}
	if spp := val(277); spp != 3 {
		return nil, fmt.Errorf("need RGB (3 samples per pixel), got %d", spp)
	}
	offsets, err := readArray(f, tags[273])
	if err != nil {
		return nil, err
	}
	m := &rgbImage{w: w, h: h, pix: make([]byte, 0, w*h*3)}
	counts, err := readArray(f, tags[279])
	if err != nil {
		return nil, err
	}
	for i, off := range offsets {
		buf := make([]byte, counts[i])
		if _, err := f.ReadAt(buf, int64(off)); err != nil {
			return nil, err
		}
		m.pix = append(m.pix, buf...)
	}
	if len(m.pix) < w*h*3 {
		return nil, fmt.Errorf("TIFF data too short (%d of %d bytes)", len(m.pix), w*h*3)
	}
	return m, nil
}

func readArray(f *os.File, e struct {
	Tag, Type uint16
	Count     uint32
	Value     uint32
}) ([]uint32, error) {
	size := 4
	if e.Type == 3 {
		size = 2
	}
	if int(e.Count)*size <= 4 {
		if e.Type == 3 {
			return []uint32{e.Value & 0xffff}, nil
		}
		return []uint32{e.Value}, nil
	}
	buf := make([]byte, int(e.Count)*size)
	if _, err := f.ReadAt(buf, int64(e.Value)); err != nil {
		return nil, err
	}
	out := make([]uint32, e.Count)
	for i := range out {
		if size == 2 {
			out[i] = uint32(binary.LittleEndian.Uint16(buf[i*2:]))
		} else {
			out[i] = binary.LittleEndian.Uint32(buf[i*4:])
		}
	}
	return out, nil
}
