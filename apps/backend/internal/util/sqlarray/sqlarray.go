// Package sqlarray contains small helpers for safely passing []string
// values to lib/pq's pq.Array on INSERT/UPDATE.
package sqlarray

// NonNilStrings returns s if non-nil, else an empty []string.
//
// pq.Array(nil) renders as SQL NULL, which (a) violates NOT NULL TEXT[]
// constraints (e.g. emergency_declarations.granted_caps,
// secret_namespaces.operations) and (b) silently changes audit semantics
// on nullable columns vs. the previous helper that emitted '{}'.
//
// Callers wrapping write-side pq.Array should pass through this helper:
//
//	pq.Array(sqlarray.NonNilStrings(req.Attributes))
func NonNilStrings(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
