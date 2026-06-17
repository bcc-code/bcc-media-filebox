// Package forms holds the hardcoded upload-form definitions. A target may
// reference a form by key (targets.form_key); when it does, uploaders must fill
// the form before uploading and the backend derives the final filename from the
// submitted values via BuildFilename.
//
// These definitions are mirrored in the frontend at frontend/src/forms/index.ts.
// Keep the two in sync — same keys, same fields, same template — when editing.
package forms

import (
	"fmt"
	"sort"
	"strings"
)

type FieldType string

const (
	FieldText   FieldType = "text"
	FieldNumber FieldType = "number"
	FieldSelect FieldType = "select"
)

// Option is one choice in a select field. Code is what lands in the filename;
// Label is what the uploader sees in the dropdown.
type Option struct {
	Code  string `json:"code"`
	Label string `json:"label"`
}

type Field struct {
	Key         string    `json:"key"`
	Label       string    `json:"label"`
	Type        FieldType `json:"type"`
	Required    bool      `json:"required"`
	MinLength   int       `json:"minLength,omitempty"`
	MaxLength   int       `json:"maxLength,omitempty"`
	Placeholder string    `json:"placeholder,omitempty"`
	Options     []Option  `json:"options,omitempty"`
	// OptionsSource names a DB-backed catalog the select options come from
	// (e.g. "projects") instead of the static Options above.
	OptionsSource string `json:"optionsSource,omitempty"`
	// Suggest enables free-text autocomplete sourced from prior uploads. The
	// suggestions are scoped by the value of the SuggestScope field (e.g. season
	// suggestions for the chosen "project").
	Suggest      bool   `json:"suggest,omitempty"`
	SuggestScope string `json:"suggestScope,omitempty"`
}

type Form struct {
	Key         string  `json:"key"`
	Label       string  `json:"label"`
	Description string  `json:"description"`
	MaxFiles    int     `json:"maxFiles"` // 1 = single file; 0 = unlimited
	Fields      []Field `json:"fields"`
	Template    string  `json:"template"` // e.g. "{arrangement}_{subEvent}_{navn}"
	// ResetFields are cleared after each successful upload (the rest persist so
	// the next file in a batch keeps shared context).
	ResetFields []string `json:"resetFields,omitempty"`
}

// Registry holds every hardcoded form keyed by Form.Key.
var Registry = map[string]Form{
	"bcc_media": {
		Key:         "bcc_media",
		Label:       "BCC Media",
		Description: "Add event details before uploading",
		MaxFiles:    1,
		Template:    "{arrangement}_{subEvent}_{navn}",
		Fields: []Field{
			{
				Key:         "arrangement",
				Label:       "Arrangement",
				Type:        FieldSelect,
				Required:    true,
				Placeholder: "Velg arrangement...",
				Options: []Option{
					{Code: "ARR", Label: "Arrangement"},
					{Code: "SMR", Label: "Sommerstevne"},
					{Code: "VIN", Label: "Vinterstevne"},
				},
			},
			{
				Key:         "subEvent",
				Label:       "Sub event",
				Type:        FieldSelect,
				Required:    true,
				Placeholder: "Velg arrangement først",
				Options: []Option{
					{Code: "SUB", Label: "Sub event"},
					{Code: "MØT", Label: "Møte"},
					{Code: "SEM", Label: "Seminar"},
				},
			},
			{
				Key:      "post",
				Label:    "Post-nr.",
				Type:     FieldNumber,
				Required: false,
			},
			{
				Key:      "type",
				Label:    "Type",
				Type:     FieldSelect,
				Required: false,
				Options: []Option{
					{Code: "", Label: "— Ingen —"},
					{Code: "VID", Label: "Video"},
					{Code: "AUD", Label: "Audio"},
				},
			},
			{
				Key:         "navn",
				Label:       "Navn",
				Type:        FieldText,
				Required:    true,
				MaxLength:   50,
				Placeholder: "For example: temafilm",
			},
		},
	},
	"camera_dailies": {
		Key:         "camera_dailies",
		Label:       "BCC Media Masters",
		Description: "Add project details before uploading",
		MaxFiles:    1,
		Template:    "{project}_{season}_{episode}_{title}",
		ResetFields: []string{"episode", "title"},
		Fields: []Field{
			{
				Key:           "project",
				Label:         "Project",
				Type:          FieldSelect,
				Required:      true,
				Placeholder:   "Select project...",
				OptionsSource: "projects",
			},
			{
				Key:          "season",
				Label:        "Season",
				Type:         FieldText,
				Required:     false,
				Suggest:      true,
				SuggestScope: "project",
			},
			{
				Key:          "episode",
				Label:        "Episode",
				Type:         FieldText,
				Required:     false,
				Suggest:      true,
				SuggestScope: "project",
			},
			{
				Key:         "title",
				Label:       "Title",
				Type:        FieldText,
				Required:    true,
				MinLength:   5,
				MaxLength:   50,
				Placeholder: "For example: cold open",
			},
		},
	},
}

// Get returns the form for key, or (zero, false) if no such form exists.
func Get(key string) (Form, bool) {
	f, ok := Registry[key]
	return f, ok
}

// Keys returns all registered form keys, sorted, for admin validation and the
// admin dropdown.
func Keys() []string {
	keys := make([]string, 0, len(Registry))
	for k := range Registry {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Validate returns an error if any required field is missing/empty or a field's
// value is shorter than its MinLength.
func Validate(f Form, values map[string]string) error {
	for _, field := range f.Fields {
		v := strings.TrimSpace(values[field.Key])
		if field.Required && v == "" {
			return fmt.Errorf("field %q is required", field.Key)
		}
		if field.MinLength > 0 && v != "" && len([]rune(v)) < field.MinLength {
			return fmt.Errorf("field %q must be at least %d characters", field.Key, field.MinLength)
		}
	}
	return nil
}

// BuildFilename resolves the form's template against the submitted values and
// appends ext. For a select field the submitted value is matched to an option
// and replaced by that option's Code; other fields are slugified. Empty optional
// tokens collapse cleanly so the name never contains "__" or a leading/trailing
// underscore. ext should include the leading dot (e.g. ".mov") or be empty.
func BuildFilename(f Form, values map[string]string, ext string) string {
	codeByKey := make(map[string]string, len(f.Fields))
	for _, field := range f.Fields {
		raw := strings.TrimSpace(values[field.Key])
		if field.Type == FieldSelect {
			codeByKey[field.Key] = optionCode(field, raw)
		} else {
			codeByKey[field.Key] = slug(raw)
		}
	}

	// Resolve {token} occurrences in order, dropping empties.
	parts := make([]string, 0, len(f.Fields))
	for _, tok := range templateTokens(f.Template) {
		if v := codeByKey[tok]; v != "" {
			parts = append(parts, v)
		}
	}
	base := strings.Join(parts, "_")
	if base == "" {
		base = "upload"
	}
	return base + ext
}

// optionCode returns the Code of the option whose Code or Label matches raw.
// Falls back to the slug of raw if nothing matches.
func optionCode(field Field, raw string) string {
	if raw == "" {
		return ""
	}
	for _, o := range field.Options {
		if o.Code == raw || o.Label == raw {
			return o.Code
		}
	}
	return slug(raw)
}

// slug keeps [A-Za-z0-9-], turns runs of anything else into a single "_", and
// trims leading/trailing underscores.
func slug(s string) string {
	var b strings.Builder
	prevUnderscore := false
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-':
			b.WriteRune(r)
			prevUnderscore = false
		default:
			if !prevUnderscore {
				b.WriteByte('_')
				prevUnderscore = true
			}
		}
	}
	return strings.Trim(b.String(), "_")
}

// templateTokens extracts the field keys referenced as {key} in template order.
func templateTokens(tmpl string) []string {
	var tokens []string
	for {
		start := strings.IndexByte(tmpl, '{')
		if start < 0 {
			break
		}
		end := strings.IndexByte(tmpl[start:], '}')
		if end < 0 {
			break
		}
		tokens = append(tokens, tmpl[start+1:start+end])
		tmpl = tmpl[start+end+1:]
	}
	return tokens
}
