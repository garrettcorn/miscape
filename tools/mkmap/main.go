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
	"sort"
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
	regions := flag.String("regions", "", "write a continents and oceans image to this path (from -mask and -names; no -src needed)")
	names := flag.String("names", "maps/earth/regions.json", "regions JSON (names and colors) for -regions")
	relief := flag.String("relief", "", "write a relief layer (grey PNG) from a shaded-relief + ocean-depth raster to this path, using -mask to tell land from sea")
	mask := flag.String("mask", "maps/earth/terrain.png", "terrain image used as the land/sea mask for -relief")
	flag.Parse()
	log.SetFlags(0)
	if *regions != "" {
		if err := makeRegions(*regions, *mask, *legendPath, *names); err != nil {
			log.Fatal(err)
		}
		return
	}
	if *src == "" {
		log.Fatal("-src is required")
	}
	img, err := readTIFF(*src)
	if err != nil {
		log.Fatal(err)
	}
	if *relief != "" {
		if err := makeRelief(img, *relief, *mask, *legendPath, *w, *h); err != nil {
			log.Fatal(err)
		}
		return
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
	spp  int    // samples per pixel: 3 (RGB) or 1 (grey)
	pix  []byte // RGB or grey
}

func (m *rgbImage) avg(x0, y0, x1, y1 int) (int, int, int) {
	var r, g, b, n int
	for y := max(y0, 0); y < min(y1, m.h); y++ {
		for x := x0; x < x1; x++ {
			xx := ((x % m.w) + m.w) % m.w
			i := (y*m.w + xx) * m.spp
			r += int(m.pix[i])
			g += int(m.pix[i+(m.spp-1)/2])
			b += int(m.pix[i+m.spp-1])
			n++
		}
	}
	if n == 0 {
		return 0, 0, 0
	}
	return r / n, g / n, b / n
}

// readTIFF reads a little-endian, uncompressed, 8-bit RGB or greyscale striped TIFF.
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
	spp := int(val(277))
	if spp == 0 {
		spp = 1 // the TIFF default
	}
	if spp != 3 && spp != 1 {
		return nil, fmt.Errorf("need RGB or grey (3 or 1 samples per pixel), got %d", spp)
	}
	offsets, err := readArray(f, tags[273])
	if err != nil {
		return nil, err
	}
	m := &rgbImage{w: w, h: h, spp: spp, pix: make([]byte, 0, w*h*spp)}
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
	if len(m.pix) < w*h*spp {
		return nil, fmt.Errorf("TIFF data too short (%d of %d bytes)", len(m.pix), w*h*spp)
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

// makeRelief writes a greyscale relief layer for the world map from a
// shaded-relief raster with ocean-bottom shading (Natural Earth's
// GRAY_50M_SR_OB). Land and sea are normalized separately so that typical
// ground sits at neutral grey (128): the game blends the layer over the
// terrain colors, lightening ridges and slopes facing the light, darkening
// shadowed slopes and deep ocean.
//
//	go run ./tools/mkmap -src GRAY_50M_SR_OB.tif -relief maps/earth/relief.png
func makeRelief(img *rgbImage, out, maskPath, legendPath string, w, h int) error {
	lb, err := os.ReadFile(legendPath)
	if err != nil {
		return err
	}
	legend, err := world.ParseLegend(lb)
	if err != nil {
		return err
	}
	mb, err := os.ReadFile(maskPath)
	if err != nil {
		return err
	}
	tm, err := world.DecodeTerrain(mb, legend)
	if err != nil {
		return err
	}
	vals := make([]int, w*h)
	sea := make([]bool, w*h)
	var land, water []int
	for y := 0; y < h; y++ {
		sy0, sy1 := y*img.h/h, (y+1)*img.h/h
		for x := 0; x < w; x++ {
			sx0, sx1 := x*img.w/w, (x+1)*img.w/w
			v, _, _ := img.avg(sx0, sy0, max(sx1, sx0+1), max(sy1, sy0+1))
			i := y*w + x
			vals[i] = v
			sea[i] = tm.At(x*tm.W/w, y*tm.H/h).IsWater()
			if sea[i] {
				water = append(water, v)
			} else {
				land = append(land, v)
			}
		}
	}
	// Map each group's 3rd..97th percentile onto 40..216 around its median.
	norm := func(s []int) func(int) uint8 {
		sort.Ints(s)
		lo, mid, hi := s[len(s)*3/100], s[len(s)/2], s[len(s)*97/100]
		return func(v int) uint8 {
			var t float64
			if v < mid {
				t = 128 - 88*float64(mid-v)/math.Max(1, float64(mid-lo))
			} else {
				t = 128 + 88*float64(v-mid)/math.Max(1, float64(hi-mid))
			}
			return uint8(math.Max(0, math.Min(255, t)))
		}
	}
	nl, nw := norm(land), norm(water)
	g := image.NewGray(image.Rect(0, 0, w, h))
	for i, v := range vals {
		if sea[i] {
			g.Pix[i] = nw(v)
		} else {
			g.Pix[i] = nl(v)
		}
	}
	f, err := os.Create(out)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := (&png.Encoder{CompressionLevel: png.BestCompression}).Encode(f, g); err != nil {
		return err
	}
	st, _ := f.Stat()
	fmt.Printf("wrote %s (%dx%d, %d KB)\n", out, w, h, st.Size()/1024)
	return nil
}

// makeRegions writes the continents-and-oceans image: each pixel of the
// terrain mask gets the color of its continent (land) or ocean (sea) from
// regions.json, decided by the boundary rules below. The result can be
// repainted by hand like the terrain.
//
//	go run ./tools/mkmap -regions maps/earth/regions.png
func makeRegions(out, maskPath, legendPath, namesPath string) error {
	lb, err := os.ReadFile(legendPath)
	if err != nil {
		return err
	}
	legend, err := world.ParseLegend(lb)
	if err != nil {
		return err
	}
	mb, err := os.ReadFile(maskPath)
	if err != nil {
		return err
	}
	tm, err := world.DecodeTerrain(mb, legend)
	if err != nil {
		return err
	}
	nb, err := os.ReadFile(namesPath)
	if err != nil {
		return err
	}
	regions, err := world.ParseRegions(nb)
	if err != nil {
		return err
	}
	pal := color.Palette{}
	idx := map[string]uint8{}
	for _, r := range regions {
		v, _ := strconv.ParseUint(strings.TrimPrefix(r.Color, "#"), 16, 32)
		idx[r.Name] = uint8(len(pal))
		pal = append(pal, color.RGBA{uint8(v >> 16), uint8(v >> 8), uint8(v), 255})
	}
	img := image.NewPaletted(image.Rect(0, 0, tm.W, tm.H), pal)
	counts := map[string]int{}
	for y := 0; y < tm.H; y++ {
		lat := 90 - (float64(y)+0.5)/float64(tm.H)*180
		for x := 0; x < tm.W; x++ {
			lon := (float64(x)+0.5)/float64(tm.W)*360 - 180
			t := tm.At(x, y)
			name := oceanAt(lat, lon)
			if !t.IsWater() {
				name = continentAt(lat, lon)
			}
			i, ok := idx[name]
			if !ok {
				return fmt.Errorf("regions.json has no %q", name)
			}
			img.SetColorIndex(x, y, i)
			counts[name]++
		}
	}
	f, err := os.Create(out)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := (&png.Encoder{CompressionLevel: png.BestCompression}).Encode(f, img); err != nil {
		return err
	}
	st, _ := f.Stat()
	fmt.Printf("wrote %s (%dx%d, %d KB)\n", out, tm.W, tm.H, st.Size()/1024)
	for _, r := range regions {
		fmt.Printf("  %-20s %5.1f%%\n", r.Name, 100*float64(counts[r.Name])/float64(tm.W*tm.H))
	}
	return nil
}

// continentAt names the continent of a land point. The boundaries follow
// the usual conventions, simplified: Europe/Asia along the Urals, the
// Caucasus and the Bosporus; Africa/Asia at Suez and the Red Sea; North/South
// America at the Panama–Colombia border; Greenland is North American, Iceland
// European, New Guinea and the Pacific islands Oceanian.
func continentAt(lat, lon float64) string {
	switch {
	case lat < -60:
		return "Antarctica"
	case lon > 165 && lat < -25, // New Zealand
		lon > 110 && lon < 155 && lat < -10, // Australia
		lon > 130 && lat < 0,                // New Guinea, Melanesia
		lon < -130 && lat < 30 && lat > -50: // Polynesia, Hawaii
		return "Australia & Oceania"
	case lon >= -25 && lon <= -12 && lat > 62 && lat < 67.5:
		return "Europe" // Iceland
	case lat > 59 && lon > -75 && lon < -10:
		return "North America" // Greenland
	case lon < -25:
		if lat < 7.2 || (lat < 12.5 && lon > -77.4) {
			return "South America"
		}
		return "North America"
	case lon > 168 && lat > 50 && lat < 56:
		return "North America" // the far end of the Aleutians (not Chukotka, further north)
	}
	// Africa: south of the Mediterranean, west of Suez and the Red Sea (which
	// runs from Suez at 29.9N 32.5E to Bab-el-Mandeb at 12.6N 43.3E).
	if lat < 37.5 && lon < 52 {
		redSea := 32.5 + (29.9-lat)/(29.9-12.6)*(43.3-32.5)
		if lat < 12.6 || lon < math.Min(redSea, 32.5+math.Max(0, 29.9-lat)) || (lat < 29.9 && lon < redSea) {
			if !(lat > 29.9 && lon > 32.4) { // Sinai and the Levant are Asian
				return "Africa"
			}
		}
	}
	// Europe: west of the Urals (60E) north of the Caucasus and Caspian
	// (about 47E between 41N and 50N), and west of the Bosporus (29E) further
	// south.
	if lat > 34 {
		switch {
		case lat >= 50 && lon < 60:
			return "Europe"
		case lat >= 41.2 && lon < 47:
			if lat < 41.2 || !(lon > 29 && lat < 42.2) { // Anatolia is Asian
				return "Europe"
			}
		case lon < 26.5:
			return "Europe"
		}
	}
	return "Asia"
}

// oceanAt names the ocean or sea of a water point.
func oceanAt(lat, lon float64) string {
	switch {
	case lat > 30 && lat < 46.5 && lon > -5.6 && lon < 36.5 && !(lat > 40.5 && lon > 27) && !(lat > 42.5 && lon < 3.2): // not the Bay of Biscay
		return "Mediterranean Sea"
	case lat > 40.5 && lat < 47.5 && lon > 27 && lon < 42:
		return "Black Sea"
	case lat > 36 && lat < 47.5 && lon > 46 && lon < 55.5:
		return "Caspian Sea"
	case lat > 66:
		return "Arctic Ocean"
	case lat < -60:
		return "Southern Ocean"
	}
	if lat < 0 {
		switch {
		case lon >= -67 && lon < 20:
			return "Atlantic Ocean"
		case lon >= 20 && lon < 147:
			if lon > 105 && lat > -8 {
				return "Pacific Ocean" // the seas north of Australia
			}
			return "Indian Ocean"
		}
		return "Pacific Ocean"
	}
	// Northern hemisphere: the Atlantic reaches from the Americas to Europe and
	// Africa, including the Caribbean, the Gulf of Mexico and Hudson Bay.
	var west float64
	switch {
	case lat < 8:
		west = -78
	case lat < 18:
		west = -87
	case lat < 31:
		west = -98
	case lat < 50:
		west = -82
	default:
		west = -100
	}
	if lon >= west && lon < 30 {
		return "Atlantic Ocean"
	}
	if lon >= 30 && lon < 105 && lat < 30 {
		return "Indian Ocean"
	}
	return "Pacific Ocean"
}
