package ui

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	gofont "github.com/go-text/typesetting/font"
	ot "github.com/go-text/typesetting/font/opentype"
	"github.com/go-text/typesetting/font/opentype/tables"
)

// SystemFont is one loadable family found on the machine.
type SystemFont struct {
	Family string
	Path   string
}

// maxFontBytes caps single files read during a scan.
const maxFontBytes = 200 << 20

// FontDirs returns the platform font locations, user locations first so
// user-installed fonts win family deduplication.
func FontDirs() []string {
	var dirs []string
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "darwin":
		if home != "" {
			dirs = append(dirs, filepath.Join(home, "Library", "Fonts"))
		}
		dirs = append(dirs,
			filepath.Join(string(filepath.Separator), "Library", "Fonts"),
			"/System/Library/Fonts",
			"/System/Library/Fonts/Supplemental",
		)
	case "windows":
		if windir := os.Getenv("WINDIR"); windir != "" {
			dirs = append(dirs, filepath.Join(windir, "Fonts"))
		}
		if home != "" {
			dirs = append(dirs, filepath.Join(home, "AppData", "Local", "Microsoft", "Windows", "Fonts"))
		}
	default: // linux and friends
		if home != "" {
			dirs = append(dirs,
				filepath.Join(home, ".fonts"),
				filepath.Join(home, ".local", "share", "fonts"),
			)
		}
		dirs = append(dirs, "/usr/share/fonts", "/usr/local/share/fonts")
	}
	return dirs
}

var fontExtensions = map[string]bool{
	".ttf": true, ".otf": true, ".ttc": true, ".dfont": true,
}

// ScanSystemFonts walks dirs and returns loadable families sorted by name.
// Unreadable, unparseable, and nameless files are skipped, so everything
// returned can actually be rendered. The first path wins per family.
func ScanSystemFonts(dirs []string) []SystemFont {
	seen := map[string]bool{}
	var out []SystemFont
	add := func(path string) {
		for _, fam := range familiesInFile(path) {
			key := strings.ToLower(fam)
			if key == "" || seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, SystemFont{Family: fam, Path: path})
		}
	}
	for _, root := range dirs {
		info, err := os.Stat(root)
		if err != nil {
			continue
		}
		if !info.IsDir() {
			add(root)
			continue
		}
		_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			if !fontExtensions[strings.ToLower(filepath.Ext(path))] {
				return nil
			}
			if info, err := d.Info(); err != nil || info.Size() > maxFontBytes {
				return nil
			}
			add(path)
			return nil
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Family) < strings.ToLower(out[j].Family)
	})
	return out
}

// familiesInFile returns the distinct family names in one font file,
// empty when the file cannot be parsed or names nothing.
func familiesInFile(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return nil
	}
	loaders, err := ot.NewLoaders(bytes.NewReader(data))
	if err != nil || len(loaders) == 0 {
		return nil
	}
	var out []string
	seen := map[string]bool{}
	for _, ld := range loaders {
		raw, err := ld.RawTable(ot.MustNewTag("name"))
		if err != nil {
			continue
		}
		names, _, err := tables.ParseName(raw)
		if err != nil {
			continue
		}
		fam := firstNonEmpty(
			names.Name(tables.NameID(16)), // typographic family
			names.Name(tables.NameID(21)), // WWS family
			names.Name(tables.NameID(1)),  // font family
		)
		fam = strings.TrimSpace(fam)
		if fam == "" || seen[strings.ToLower(fam)] {
			continue
		}
		seen[strings.ToLower(fam)] = true
		out = append(out, fam)
	}
	return out
}

// LoadFontFace returns standalone single-font bytes Fyne can render:
// single fonts pass through, collections yield the named family's first
// face repacked as its own sfnt. Validation mirrors Fyne's loader, so a
// returned value can never crash the renderer.
func LoadFontFace(path, family string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ExtractFace(data, family)
}

// LoadFontResource reads path and returns its bytes when the file parses
// as at least one loadable face.
func LoadFontResource(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	loaders, err := ot.NewLoaders(bytes.NewReader(data))
	if err != nil || len(loaders) == 0 {
		return nil, errFontUnloadable(path)
	}
	return data, nil
}

// ExtractFace resolves data to single-font bytes. Single fonts pass
// through untouched. For collections, the first face matching family
// (case-insensitive; empty matches the first named face) is repacked as
// a standalone sfnt with corrected checksums.
func ExtractFace(data []byte, family string) ([]byte, error) {
	if _, err := gofont.ParseTTF(bytes.NewReader(data)); err == nil {
		return data, nil
	}
	loaders, err := ot.NewLoaders(bytes.NewReader(data))
	if err != nil || len(loaders) == 0 {
		return nil, errFontUnloadable("<memory>")
	}
	for _, ld := range loaders {
		if fam := loaderFamily(ld); fam != "" && (family == "" || strings.EqualFold(fam, family)) {
			return repackFace(ld)
		}
	}
	return nil, fmt.Errorf("family %q not found in font collection", family)
}

func loaderFamily(ld *ot.Loader) string {
	raw, err := ld.RawTable(ot.MustNewTag("name"))
	if err != nil {
		return ""
	}
	names, _, err := tables.ParseName(raw)
	if err != nil {
		return ""
	}
	return firstNonEmpty(
		names.Name(tables.NameID(16)),
		names.Name(tables.NameID(21)),
		names.Name(tables.NameID(1)),
	)
}

// repackFace serializes one collection face as a standalone sfnt file.
func repackFace(ld *ot.Loader) ([]byte, error) {
	type entry struct {
		tag  uint32
		data []byte
	}
	var tabs []entry
	for _, tag := range ld.Tables() {
		raw, err := ld.RawTable(tag)
		if err != nil || len(raw) == 0 {
			continue
		}
		data := bytes.Clone(raw)
		if tag == ot.MustNewTag("head") && len(data) >= 12 {
			// checkSumAdjustment recomputed below.
			data[8], data[9], data[10], data[11] = 0, 0, 0, 0
		}
		tabs = append(tabs, entry{tag: uint32(tag), data: data})
	}
	if len(tabs) == 0 {
		return nil, fmt.Errorf("face has no tables")
	}
	sort.Slice(tabs, func(i, j int) bool { return tabs[i].tag < tabs[j].tag })

	n := len(tabs)
	headerLen := 12 + 16*n
	off := headerLen
	var offsets []int
	for _, tb := range tabs {
		offsets = append(offsets, off)
		off += (len(tb.data) + 3) &^ 3
	}
	out := make([]byte, off)
	binary.BigEndian.PutUint32(out[0:], scalerFor(ld.Type))
	binary.BigEndian.PutUint16(out[4:], uint16(n))
	// searchRange/entrySelector/rangeShift for max power of 2 <= n.
	p := 1
	for p*2 <= n {
		p *= 2
	}
	entrySelector := 0
	for q := p; q > 1; q >>= 1 {
		entrySelector++
	}
	binary.BigEndian.PutUint16(out[6:], uint16(p*16))
	binary.BigEndian.PutUint16(out[8:], uint16(entrySelector))
	binary.BigEndian.PutUint16(out[10:], uint16(n*16-p*16))
	var total uint32
	for i, tb := range tabs {
		rec := out[12+16*i:]
		binary.BigEndian.PutUint32(rec[0:], tb.tag)
		binary.BigEndian.PutUint32(rec[8:], uint32(offsets[i]))
		binary.BigEndian.PutUint32(rec[12:], uint32(len(tb.data)))
		copy(out[offsets[i]:], tb.data)
		sum := checksum(out[offsets[i] : offsets[i]+((len(tb.data)+3)&^3)])
		binary.BigEndian.PutUint32(rec[4:], sum)
		total += sum
	}
	// Whole-file checksum for head.checkSumAdjustment.
	total += checksum(out)
	const magic = 0xB1B0AFBA
	for i, tb := range tabs {
		if tb.tag == uint32(ot.MustNewTag("head")) {
			binary.BigEndian.PutUint32(out[offsets[i]+8:], magic-total)
			break
		}
	}
	return out, nil
}

func scalerFor(t ot.Tag) uint32 {
	switch t {
	case ot.MustNewTag("OTTO"), ot.MustNewTag("typ1"):
		return uint32(t)
	default:
		return 0x00010000
	}
}

func checksum(b []byte) uint32 {
	var sum uint32
	for i := 0; i < len(b); i += 4 {
		sum += binary.BigEndian.Uint32(b[i:])
	}
	return sum
}

type unloadableError struct{ path string }

func (e unloadableError) Error() string { return "font cannot be loaded: " + e.path }

func errFontUnloadable(path string) error { return unloadableError{path} }

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}
