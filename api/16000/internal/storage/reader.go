package storage

import "sort"

// CloneMap makes a shallow copy of a map.
func CloneMap(src map[string]any) map[string]any {
	if src == nil {
		return nil
	}
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// ItemsToSlice converts an "items" map (object-of-objects) into a stable, deterministic slice.
//
// Expected shape:
//
//	items: { "ID1": { ... }, "ID2": { ... } }
//
// It adds the key as field "id" on each returned object.
func ItemsToSlice(items map[string]any) []map[string]any {
	if items == nil {
		return []map[string]any{}
	}

	keys := make([]string, 0, len(items))
	for k := range items {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	out := make([]map[string]any, 0, len(keys))
	for _, id := range keys {
		raw, ok := items[id]
		if !ok || raw == nil {
			continue
		}
		m, ok := raw.(map[string]any)
		if !ok {
			// If the stored value isn't an object, skip.
			continue
		}

		clone := CloneMap(m)
		// Reserve "id" for the object key.
		clone["id"] = id
		out = append(out, clone)
	}

	return out
}

// FilterByStringField filters a slice of objects by a string field equality.
// If the field is missing or not a string, the item is excluded.
func FilterByStringField(items []map[string]any, field, value string) []map[string]any {
	if value == "" {
		return []map[string]any{}
	}

	out := make([]map[string]any, 0)
	for _, it := range items {
		v, ok := it[field]
		if !ok {
			continue
		}
		s, ok := v.(string)
		if !ok {
			continue
		}
		if s == value {
			out = append(out, it)
		}
	}
	return out
}
