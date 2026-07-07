package catalog

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type IconResolution struct {
	Path string
}

type IconResolver struct {
	ThemeRoots  []string
	PixmapRoots []string
	cache       map[string]IconResolution
}

func NewIconResolver(themeRoots []string, pixmapRoots []string) *IconResolver {
	return &IconResolver{
		ThemeRoots:  cleanRoots(themeRoots),
		PixmapRoots: cleanRoots(pixmapRoots),
		cache:       map[string]IconResolution{},
	}
}

func DefaultIconThemeRoots(home string) []string {
	roots := []string{}
	if strings.TrimSpace(home) != "" {
		roots = append(roots, filepath.Join(home, ".local/share/icons"))
	}
	if dataHome := strings.TrimSpace(os.Getenv("XDG_DATA_HOME")); dataHome != "" {
		roots = append(roots, filepath.Join(dataHome, "icons"))
	}
	for _, root := range filepath.SplitList(os.Getenv("XDG_DATA_DIRS")) {
		if strings.TrimSpace(root) != "" {
			roots = append(roots, filepath.Join(root, "icons"))
		}
	}
	roots = append(roots, "/usr/local/share/icons", "/usr/share/icons")
	return cleanRoots(roots)
}

func DefaultIconPixmapRoots(home string) []string {
	roots := []string{}
	if strings.TrimSpace(home) != "" {
		roots = append(roots, filepath.Join(home, ".local/share/pixmaps"))
	}
	if dataHome := strings.TrimSpace(os.Getenv("XDG_DATA_HOME")); dataHome != "" {
		roots = append(roots, filepath.Join(dataHome, "pixmaps"))
	}
	for _, root := range filepath.SplitList(os.Getenv("XDG_DATA_DIRS")) {
		if strings.TrimSpace(root) != "" {
			roots = append(roots, filepath.Join(root, "pixmaps"))
		}
	}
	roots = append(roots, "/usr/local/share/pixmaps", "/usr/share/pixmaps")
	return cleanRoots(roots)
}

func (resolver *IconResolver) Resolve(icon string) IconResolution {
	icon = strings.TrimSpace(icon)
	if icon == "" {
		return IconResolution{}
	}
	if resolver == nil {
		return IconResolution{}
	}
	if cached, ok := resolver.cache[icon]; ok {
		return cached
	}
	resolution := resolver.resolve(icon)
	resolver.cache[icon] = resolution
	return resolution
}

func (resolver *IconResolver) resolve(icon string) IconResolution {
	if filepath.IsAbs(icon) {
		if fileExists(icon) {
			return IconResolution{Path: icon}
		}
		return IconResolution{}
	}
	if strings.ContainsAny(icon, `/\`) {
		return IconResolution{}
	}

	names := iconNames(icon)
	for _, root := range resolver.PixmapRoots {
		for _, name := range names {
			if path := firstExistingIconPath(root, name); path != "" {
				return IconResolution{Path: path}
			}
		}
	}
	for _, root := range resolver.ThemeRoots {
		themeEntries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		sort.Slice(themeEntries, func(left, right int) bool {
			return themeRank(themeEntries[left].Name()) < themeRank(themeEntries[right].Name())
		})
		for _, themeEntry := range themeEntries {
			if !themeEntry.IsDir() {
				continue
			}
			themeRoot := filepath.Join(root, themeEntry.Name())
			for _, size := range iconSizes {
				for _, context := range iconContexts {
					dir := filepath.Join(themeRoot, size, context)
					for _, name := range names {
						if path := firstExistingIconPath(dir, name); path != "" {
							return IconResolution{Path: path}
						}
					}
				}
			}
		}
	}
	return IconResolution{}
}

func cleanRoots(values []string) []string {
	seen := map[string]bool{}
	roots := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		roots = append(roots, value)
	}
	return roots
}

func iconNames(icon string) []string {
	ext := strings.ToLower(filepath.Ext(icon))
	if ext == ".png" || ext == ".svg" || ext == ".xpm" || ext == ".jpg" || ext == ".jpeg" || ext == ".webp" {
		return []string{icon}
	}
	names := make([]string, 0, len(iconExtensions))
	for _, extension := range iconExtensions {
		names = append(names, icon+extension)
	}
	return names
}

func firstExistingIconPath(dir string, name string) string {
	path := filepath.Join(dir, name)
	if fileExists(path) {
		return path
	}
	return ""
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func themeRank(name string) int {
	switch strings.ToLower(name) {
	case "hicolor":
		return 0
	case "adwaita":
		return 1
	default:
		return 10
	}
}

var iconExtensions = []string{".svg", ".png", ".xpm", ".webp", ".jpg", ".jpeg"}

var iconSizes = []string{
	"scalable",
	"symbolic",
	"512x512",
	"256x256",
	"128x128",
	"96x96",
	"64x64",
	"48x48",
	"32x32",
	"24x24",
	"22x22",
	"16x16",
}

var iconContexts = []string{
	"apps",
	"categories",
	"devices",
	"mimetypes",
	"places",
	"status",
	"actions",
}
