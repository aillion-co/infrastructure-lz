package kcc

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"unicode"
)

// User-supplied strings are interpolated into text/template-rendered KCC
// manifests, which perform no escaping. Without validation a value
// containing a newline could inject an entirely new YAML document into
// privileged infrastructure-as-code, and a value containing YAML
// structural characters (e.g. ": ") could corrupt the manifest. The
// helpers here validate every user string before it reaches a template.

var (
	// resourceNamePattern matches GCP-style resource names and IDs:
	// lowercase letters, digits, and hyphens, starting with a letter.
	resourceNamePattern = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	// tokenPattern matches billing-account-style tokens (upper/lower/digit
	// and hyphen), e.g. AAAAAA-BBBBBB-CCCCCC.
	tokenPattern = regexp.MustCompile(`^[A-Za-z0-9-]+$`)
	// digitsPattern matches numeric IDs (organization/folder IDs).
	digitsPattern = regexp.MustCompile(`^[0-9]+$`)
	// modelPattern matches model identifiers, which may contain dots and
	// slashes (e.g. gemini-2.0-flash) but no YAML-structural characters.
	modelPattern = regexp.MustCompile(`^[A-Za-z0-9._/-]+$`)
	// labelPattern matches BigQuery dataset IDs and similar identifiers that
	// permit underscores (e.g. customer_events).
	labelPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
)

// decodeConfig decodes an arbitrary config payload (typically a
// map[string]interface{} from JSON) into the typed config T, then rejects
// any string field containing control characters. This is the shared
// front door for every feature builder's config: it removes the repeated
// marshal/unmarshal boilerplate and guarantees no field can carry a
// newline or other control character into a template.
func decodeConfig[T any](feature string, config interface{}) (*T, error) {
	cfg, ok := config.(*T)
	if !ok {
		data, err := json.Marshal(config)
		if err != nil {
			return nil, fmt.Errorf("marshalling %s config: %w", feature, err)
		}
		cfg = new(T)
		if err := json.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("unmarshalling %s config: %w", feature, err)
		}
	}
	if err := rejectControlChars(feature, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// rejectControlChars walks the exported string fields of a struct (and
// string slice elements) and returns a ValidationError if any contains a
// control character such as a newline or carriage return. This is the
// backstop against YAML document injection for every field, including
// free-text fields that are otherwise only YAML-quoted.
func rejectControlChars(feature string, v interface{}) error {
	rv := reflect.ValueOf(v)
	for rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return nil
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return nil
	}
	rt := rv.Type()
	for i := 0; i < rv.NumField(); i++ {
		if rt.Field(i).PkgPath != "" {
			continue // unexported
		}
		field := rv.Field(i)
		jsonName := jsonFieldName(rt.Field(i))
		switch field.Kind() {
		case reflect.String:
			if err := checkNoControl(feature, jsonName, field.String()); err != nil {
				return err
			}
		case reflect.Slice:
			if field.Type().Elem().Kind() == reflect.String {
				for j := 0; j < field.Len(); j++ {
					if err := checkNoControl(feature, jsonName, field.Index(j).String()); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

func checkNoControl(feature, field, val string) error {
	for _, r := range val {
		if r == '\n' || r == '\r' || (unicode.IsControl(r) && r != '\t') {
			return newValidationError(feature, field, "must not contain control characters or line breaks")
		}
	}
	return nil
}

func jsonFieldName(f reflect.StructField) string {
	tag := f.Tag.Get("json")
	if tag == "" {
		return f.Name
	}
	if comma := strings.IndexByte(tag, ','); comma >= 0 {
		tag = tag[:comma]
	}
	if tag == "" || tag == "-" {
		return f.Name
	}
	return tag
}

// requireName validates a required GCP-style resource name/ID field.
func requireName(feature, field, val string) error {
	if val == "" {
		return newValidationError(feature, field, "is required")
	}
	if !resourceNamePattern.MatchString(val) {
		return newValidationError(feature, field, "must be lowercase letters, digits, and hyphens, starting with a letter")
	}
	return nil
}

// requireToken validates a required token field (e.g. billing account).
func requireToken(feature, field, val string) error {
	if val == "" {
		return newValidationError(feature, field, "is required")
	}
	if !tokenPattern.MatchString(val) {
		return newValidationError(feature, field, "must contain only letters, digits, and hyphens")
	}
	return nil
}

// requireDigits validates a required numeric ID field.
func requireDigits(feature, field, val string) error {
	if val == "" {
		return newValidationError(feature, field, "is required")
	}
	if !digitsPattern.MatchString(val) {
		return newValidationError(feature, field, "must be numeric")
	}
	return nil
}

// validateModel validates an optional model identifier field.
func validateModel(feature, field, val string) error {
	if val == "" {
		return nil
	}
	if !modelPattern.MatchString(val) {
		return newValidationError(feature, field, "must contain only letters, digits, and . _ / - characters")
	}
	return nil
}

// validateOptionalName validates an optional GCP-style name field.
func validateOptionalName(feature, field, val string) error {
	if val == "" {
		return nil
	}
	return requireName(feature, field, val)
}

// validateOptionalID validates an optional identifier that permits
// underscores (e.g. a BigQuery dataset ID).
func validateOptionalID(feature, field, val string) error {
	if val == "" {
		return nil
	}
	if !labelPattern.MatchString(val) {
		return newValidationError(feature, field, "must contain only letters, digits, underscores, and hyphens")
	}
	return nil
}

// validateCSVNames validates that every comma-separated element is a valid
// GCP-style name. Empty input is rejected (use before checking presence).
func validateCSVNames(feature, field, val string) error {
	parts := strings.Split(val, ",")
	found := false
	for _, p := range parts {
		t := strings.TrimSpace(p)
		if t == "" {
			continue
		}
		found = true
		if !resourceNamePattern.MatchString(t) {
			return newValidationError(feature, field,
				fmt.Sprintf("entry %q must be lowercase letters, digits, and hyphens, starting with a letter", t))
		}
	}
	if !found {
		return newValidationError(feature, field, "is required")
	}
	return nil
}

// yamlScalar renders a string as a safe double-quoted YAML scalar. Used
// for free-text and URL fields that legitimately contain spaces, colons,
// or other YAML-structural characters.
func yamlScalar(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}
