package ui

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

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
