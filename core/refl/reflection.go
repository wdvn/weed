package refl

import (
	"reflect"
	"strings"
)

// ExtractFields extracts the values of struct fields that have a "json" tag matching any of the targetTags.
// It accepts a struct or a pointer to a struct. Unexported fields, fields without a "json" tag,
// and fields with a json tag of "-" are ignored.
// It correctly handles json tag options like "omitempty" by only considering the name part of the tag.
func ExtractFields(input any, targetTags []string) map[string]any {
	result := make(map[string]any)
	tagMap := make(map[string]bool)
	for _, tag := range targetTags {
		tagMap[tag] = true
	}

	val := reflect.ValueOf(input)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		return result
	}

	typ := val.Type()
	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)
		fieldVal := val.Field(i)

		// 1. Check access rights (must be an exported field)
		if !fieldVal.CanInterface() {
			continue
		}

		// 2. Get the value of the "json" tag
		tag := field.Tag.Get("json")

		// 3. Handle json:"-" case
		if tag == "-" {
			continue // Completely ignore according to Go JSON specification
		}

		// 4. Handle no tag case (tag == "")
		// If there is no tag, we have 2 options:
		// - Ignore (since you want to get by tag)
		// - Or get by Field name (depending on your needs)
		cleanTag := tag
		if tag == "" {
			// cleanTag = field.Name // Uncomment if you want to use the field name when the tag is missing
			continue
		}

		// Extract the main name before the comma (e.g., "user_id,omitempty" -> "user_id")
		if idx := strings.Index(tag, ","); idx != -1 {
			cleanTag = tag[:idx]
		}

		// 5. Check if the tag is in the list to be extracted
		if tagMap[cleanTag] {
			result[cleanTag] = fieldVal.Interface()
		}
	}

	return result
}
