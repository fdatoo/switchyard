package dashboard

// BuiltinClassIDs are the eight built-in widget class identifiers.
var BuiltinClassIDs = []string{
	"EntityToggle",
	"Gauge",
	"LineChart",
	"CameraStream",
	"Markdown",
	"ScriptButton",
	"EntityList",
	"GroupCard",
}

// InstalledPack represents a single installed widget pack.
type InstalledPack struct {
	Name    string
	Version string
	Classes []PackClass
}

// PackClass is a single widget class exported by a pack.
type PackClass struct {
	Name       string
	BundleURL  string
	BundleHash string
}

// WidgetClassInfo describes one widget class available on the server.
type WidgetClassInfo struct {
	ClassID     string
	IsContainer bool
	IsBuiltin   bool
	PackName    string
	PackVersion string
	BundleURL   string
	BundleHash  string
}

// Catalog is the server-side widget class registry.
type Catalog struct {
	packs []InstalledPack
}

// NewCatalog creates a Catalog with built-ins plus any installed packs.
func NewCatalog(packs []InstalledPack) *Catalog {
	return &Catalog{packs: packs}
}

// WidgetClasses returns all available widget classes.
func (c *Catalog) WidgetClasses() []WidgetClassInfo {
	out := make([]WidgetClassInfo, 0, len(BuiltinClassIDs)+8)
	for _, id := range BuiltinClassIDs {
		out = append(out, WidgetClassInfo{
			ClassID:     id,
			IsBuiltin:   true,
			IsContainer: id == "GroupCard",
		})
	}
	for _, p := range c.packs {
		for _, cls := range p.Classes {
			out = append(out, WidgetClassInfo{
				ClassID:     p.Name + "/" + cls.Name,
				IsBuiltin:   false,
				IsContainer: false,
				PackName:    p.Name,
				PackVersion: p.Version,
				BundleURL:   cls.BundleURL,
				BundleHash:  cls.BundleHash,
			})
		}
	}
	return out
}

// LookupClass finds a class by its full classID (e.g. "EntityToggle" or "bar-widgets/BarChart").
func (c *Catalog) LookupClass(classID string) *WidgetClassInfo {
	for _, wc := range c.WidgetClasses() {
		wc := wc
		if wc.ClassID == classID {
			return &wc
		}
	}
	return nil
}
