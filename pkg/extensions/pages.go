package extensions

import (
	"encoding/json"
	"fmt"

	goflowmodel "github.com/pflow-xyz/go-pflow/metamodel"
)

const (
	// PagesExtensionName is the extension name for page definitions.
	PagesExtensionName = "petri-pilot/pages"
)

// PageExtension adds UI page definitions to a Petri net model.
type PageExtension struct {
	goflowmodel.BaseExtension
	Pages      []Page      `json:"pages"`
	Navigation *Navigation `json:"navigation,omitempty"`
}

// Page defines a UI page.
type Page struct {
	ID          string `json:"id"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Path        string `json:"path"` // URL path
	Icon        string `json:"icon,omitempty"`

	// Layout defines the page structure.
	Layout PageLayout `json:"layout"`

	// Access defines who can view this page.
	Access []string `json:"access,omitempty"` // Role IDs
}

// PageLayout defines the structure of a page.
type PageLayout struct {
	Type       string        `json:"type"` // list, detail, form, dashboard, custom
	Entity     string        `json:"entity,omitempty"`
	Components []UIComponent `json:"components,omitempty"`
}

// UIComponent defines a UI component within a page.
type UIComponent struct {
	Type   string         `json:"type"` // table, card, form, chart, stat, custom
	Config map[string]any `json:"config,omitempty"`
}

// Navigation represents the navigation menu configuration.
type Navigation struct {
	Brand string           `json:"brand"`
	Items []NavigationItem `json:"items"`
}

// NavigationItem represents a single navigation menu item.
type NavigationItem struct {
	Label string   `json:"label"`
	Path  string   `json:"path"`
	Icon  string   `json:"icon,omitempty"`
	Roles []string `json:"roles,omitempty"` // empty = visible to all
}

// NewPageExtension creates a new PageExtension.
func NewPageExtension() *PageExtension {
	return &PageExtension{
		BaseExtension: goflowmodel.NewBaseExtension(PagesExtensionName),
		Pages:         make([]Page, 0),
	}
}

// Validate checks that all pages are valid.
func (p *PageExtension) Validate(model *goflowmodel.Model) error {
	seenIDs := make(map[string]bool)
	seenPaths := make(map[string]bool)

	for _, page := range p.Pages {
		if seenIDs[page.ID] {
			return fmt.Errorf("duplicate page ID: %s", page.ID)
		}
		seenIDs[page.ID] = true

		if seenPaths[page.Path] {
			return fmt.Errorf("duplicate page path: %s", page.Path)
		}
		seenPaths[page.Path] = true

		// Validate layout type
		validTypes := map[string]bool{
			"list": true, "detail": true, "form": true,
			"dashboard": true, "custom": true,
		}
		if !validTypes[page.Layout.Type] && page.Layout.Type != "" {
			return fmt.Errorf("page %s: invalid layout type: %s", page.ID, page.Layout.Type)
		}
	}

	return nil
}

// MarshalJSON serializes the pages and navigation.
func (p *PageExtension) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Pages      []Page      `json:"pages"`
		Navigation *Navigation `json:"navigation,omitempty"`
	}{
		Pages:      p.Pages,
		Navigation: p.Navigation,
	})
}

// UnmarshalJSON deserializes the pages and navigation.
func (p *PageExtension) UnmarshalJSON(data []byte) error {
	var raw struct {
		Pages      []Page      `json:"pages"`
		Navigation *Navigation `json:"navigation,omitempty"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	p.Pages = raw.Pages
	p.Navigation = raw.Navigation
	return nil
}

// AddPage adds a page to the extension.
func (p *PageExtension) AddPage(page Page) {
	p.Pages = append(p.Pages, page)
}

// PageByID returns a page by ID, or nil if not found.
func (p *PageExtension) PageByID(id string) *Page {
	for i := range p.Pages {
		if p.Pages[i].ID == id {
			return &p.Pages[i]
		}
	}
	return nil
}

// PageByPath returns a page by path, or nil if not found.
func (p *PageExtension) PageByPath(path string) *Page {
	for i := range p.Pages {
		if p.Pages[i].Path == path {
			return &p.Pages[i]
		}
	}
	return nil
}

// SetNavigation sets the navigation configuration.
func (p *PageExtension) SetNavigation(nav Navigation) {
	p.Navigation = &nav
}

// init registers the page extension with the default registry.
func init() {
	goflowmodel.Register(PagesExtensionName, func() goflowmodel.ModelExtension {
		return NewPageExtension()
	})
}
