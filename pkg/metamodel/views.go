package metamodel

// View represents a grouping of fields for UI rendering.
// Views define how data should be presented in forms, cards, or other UI components.
type View struct {
	ID          string      `json:"id"`
	Name        string      `json:"name,omitempty"`
	Description string      `json:"description,omitempty"`
	Kind        ViewKind    `json:"kind,omitempty"` // form, card, table, detail
	Groups      []ViewGroup `json:"groups,omitempty"`
	Actions     []string    `json:"actions,omitempty"` // Action IDs that can be triggered from this view
}

// ViewKind specifies the type of view.
type ViewKind string

const (
	// ViewKindForm is an input form for collecting data.
	ViewKindForm ViewKind = "form"

	// ViewKindCard is a summary card displaying key information.
	ViewKindCard ViewKind = "card"

	// ViewKindTable is a tabular view for lists of records.
	ViewKindTable ViewKind = "table"

	// ViewKindDetail is a detailed view showing all fields.
	ViewKindDetail ViewKind = "detail"
)

// ViewGroup represents a logical grouping of fields within a view.
type ViewGroup struct {
	ID          string      `json:"id"`
	Name        string      `json:"name,omitempty"`
	Description string      `json:"description,omitempty"`
	Fields      []ViewField `json:"fields"`
	Collapsible bool        `json:"collapsible,omitempty"`
	Collapsed   bool        `json:"collapsed,omitempty"` // Initial state if collapsible
}

// ViewField represents a single field within a view group.
type ViewField struct {
	// Binding is the field name in the bindings/state (e.g., "from", "amount").
	Binding string `json:"binding"`

	// Label is the display label for the field.
	Label string `json:"label,omitempty"`

	// Description provides help text for the field.
	Description string `json:"description,omitempty"`

	// FieldType specifies the input/display type.
	FieldType FieldType `json:"type,omitempty"`

	// Required indicates if this field is required for form submission.
	Required bool `json:"required,omitempty"`

	// ReadOnly indicates if this field is display-only.
	ReadOnly bool `json:"readonly,omitempty"`

	// Hidden indicates if this field should be hidden.
	Hidden bool `json:"hidden,omitempty"`

	// Validation contains validation rules for the field.
	Validation *FieldValidation `json:"validation,omitempty"`

	// Options contains allowed values for select/radio fields.
	Options []FieldOption `json:"options,omitempty"`

	// Placeholder is hint text shown in empty fields.
	Placeholder string `json:"placeholder,omitempty"`

	// DefaultValue is the initial value for new records.
	DefaultValue any `json:"default,omitempty"`

	// Width specifies the field width (e.g., "full", "half", "third").
	Width string `json:"width,omitempty"`

	// Order specifies the display order within the group.
	Order int `json:"order,omitempty"`
}

// FieldType specifies how a field should be rendered.
type FieldType string

const (
	FieldTypeText     FieldType = "text"
	FieldTypeNumber   FieldType = "number"
	FieldTypeEmail    FieldType = "email"
	FieldTypePassword FieldType = "password"
	FieldTypeTextarea FieldType = "textarea"
	FieldTypeSelect   FieldType = "select"
	FieldTypeRadio    FieldType = "radio"
	FieldTypeCheckbox FieldType = "checkbox"
	FieldTypeDate     FieldType = "date"
	FieldTypeDateTime FieldType = "datetime"
	FieldTypeTime     FieldType = "time"
	FieldTypeFile     FieldType = "file"
	FieldTypeHidden   FieldType = "hidden"
	FieldTypeAddress  FieldType = "address" // Blockchain/wallet address
	FieldTypeAmount   FieldType = "amount"  // Numeric amount with formatting
	FieldTypeCurrency FieldType = "currency"
	FieldTypeJSON     FieldType = "json" // JSON editor
)

// FieldValidation contains validation rules for a field.
type FieldValidation struct {
	// Min value for numeric fields, min length for strings.
	Min *float64 `json:"min,omitempty"`

	// Max value for numeric fields, max length for strings.
	Max *float64 `json:"max,omitempty"`

	// Pattern is a regex pattern for validation.
	Pattern string `json:"pattern,omitempty"`

	// Message is a custom error message.
	Message string `json:"message,omitempty"`

	// Custom is a guard expression for custom validation.
	// e.g., "amount > 0 && amount <= balance"
	Custom string `json:"custom,omitempty"`
}

// FieldOption represents an option for select/radio fields.
type FieldOption struct {
	Value    any    `json:"value"`
	Label    string `json:"label"`
	Disabled bool   `json:"disabled,omitempty"`
}

// FormSpec represents a complete form specification derived from an action.
type FormSpec struct {
	// ActionID is the action this form submits to.
	ActionID string `json:"action_id"`

	// Title is the form title.
	Title string `json:"title,omitempty"`

	// Description provides form instructions.
	Description string `json:"description,omitempty"`

	// Groups organize the form fields.
	Groups []ViewGroup `json:"groups"`

	// SubmitLabel is the text for the submit button.
	SubmitLabel string `json:"submit_label,omitempty"`

	// CancelLabel is the text for the cancel button.
	CancelLabel string `json:"cancel_label,omitempty"`
}

// GenerateFormSpec creates a form specification from an action's bindings.
func GenerateFormSpec(action *Action, stateMap map[string]*State) *FormSpec {
	if action == nil {
		return nil
	}

	form := &FormSpec{
		ActionID:    action.ID,
		Title:       toPascalCase(action.ID),
		Description: action.Description,
		SubmitLabel: toPascalCase(action.ID),
		CancelLabel: "Cancel",
	}

	// Create default group with all bindings
	group := ViewGroup{
		ID:     "default",
		Name:   "Details",
		Fields: make([]ViewField, 0),
	}

	order := 0
	for bindingName, bindingType := range action.Bindings {
		field := ViewField{
			Binding:  bindingName,
			Label:    toPascalCase(bindingName),
			Required: true,
			Order:    order,
		}

		// Determine field type based on binding type
		switch bindingType {
		case "address":
			field.FieldType = FieldTypeAddress
			field.Placeholder = "0x..."
		case "amount", "int64", "int", "uint256":
			field.FieldType = FieldTypeAmount
			field.Validation = &FieldValidation{
				Min: ptrFloat64(0),
			}
		case "string":
			field.FieldType = FieldTypeText
		case "bool":
			field.FieldType = FieldTypeCheckbox
			field.Required = false
		default:
			field.FieldType = FieldTypeText
		}

		group.Fields = append(group.Fields, field)
		order++
	}

	form.Groups = []ViewGroup{group}
	return form
}

// toPascalCase converts a snake_case or kebab-case string to Pascal Case with spaces.
func toPascalCase(s string) string {
	if s == "" {
		return ""
	}

	result := ""
	capitalizeNext := true

	for _, r := range s {
		if r == '_' || r == '-' {
			result += " "
			capitalizeNext = true
			continue
		}

		if capitalizeNext && r >= 'a' && r <= 'z' {
			result += string(r - 32)
			capitalizeNext = false
		} else if capitalizeNext && r >= 'A' && r <= 'Z' {
			result += string(r)
			capitalizeNext = false
		} else {
			result += string(r)
		}
	}

	return result
}

func ptrFloat64(v float64) *float64 {
	return &v
}

// AddView adds a view to the schema.
func (s *Schema) AddView(v View) *Schema {
	s.Views = append(s.Views, v)
	return s
}

// ViewByID returns a view by its ID, or nil if not found.
func (s *Schema) ViewByID(id string) *View {
	for i := range s.Views {
		if s.Views[i].ID == id {
			return &s.Views[i]
		}
	}
	return nil
}
