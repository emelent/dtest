package dotnet

import "strings"

// filterEscaper escapes the characters that are special in a VSTest filter
// expression so they match literally.
var filterEscaper = strings.NewReplacer(
	`\`, `\\`, `(`, `\(`, `)`, `\)`, `&`, `\&`, `|`, `\|`, `=`, `\=`, `!`, `\!`, `~`, `\~`,
)

// FilterExact is a --filter expression matching the test method with fully
// qualified name fqn, which for a theory means every one of its rows.
func FilterExact(fqn string) string {
	return "FullyQualifiedName=" + filterEscaper.Replace(fqn)
}

// FilterPrefix is a --filter expression matching every test whose fully
// qualified name contains prefix; callers pass "Ns.Class." to select a class
// and "Ns." for a namespace.
func FilterPrefix(prefix string) string {
	return "FullyQualifiedName~" + filterEscaper.Replace(prefix)
}

// JoinFilters combines filter expressions so a test matching any is run.
func JoinFilters(filters []string) string {
	return strings.Join(filters, "|")
}
