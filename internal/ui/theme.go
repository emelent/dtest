package ui

import (
	"fmt"
	"image/color"
	"sort"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
)

// colorRoles are the palette entries a config file may set, by the role
// each plays rather than by the colour it happens to be. Foregrounds point
// at the palette variable they set; the three backgrounds are handled
// separately, since they are kept as raw escapes.
var colorRoles = map[string]*color.Color{
	"passed":  &colGreen,
	"failed":  &colRed,
	"skipped": &colYellow,
	"running": &colCyan,
	"dim":     &colGrey,
	"badge":   &colWhite,
}

// backgroundRoles are the palette entries painted behind a whole line.
var backgroundRoles = map[string]*string{
	"cursor":           &bgFocused,
	"cursor-unfocused": &bgUnfocused,
	"selection":        &bgSelected,
}

// ApplyColors repaints the palette from a config's [colors] table and
// rebuilds every style from it. An unknown role or an unreadable colour is
// an error and nothing is changed, so a typo cannot leave half a theme.
func ApplyColors(colors map[string]string) error {
	if len(colors) == 0 {
		return nil
	}
	fg := map[*color.Color]color.Color{}
	bg := map[*string]string{}
	for _, role := range sortedKeys(colors) {
		v := colors[role]
		switch {
		case colorRoles[role] != nil:
			if !validColor(v) {
				return fmt.Errorf("colors.%s: %q is not a colour", role, v)
			}
			fg[colorRoles[role]] = lipgloss.Color(v)
		case backgroundRoles[role] != nil:
			if !validColor(v) {
				return fmt.Errorf("colors.%s: %q is not a colour", role, v)
			}
			bg[backgroundRoles[role]] = bgSeq(v)
		default:
			return fmt.Errorf("colors.%s: no such colour, try one of %s", role, strings.Join(colorNames(), ", "))
		}
	}
	for p, v := range fg {
		*p = v
	}
	for p, v := range bg {
		*p = v
	}
	buildStyles()
	return nil
}

// colorNames lists every role a config may set, for an error message.
func colorNames() []string {
	names := make([]string, 0, len(colorRoles)+len(backgroundRoles))
	for name := range colorRoles {
		names = append(names, name)
	}
	for name := range backgroundRoles {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// validColor reports whether v is a 256-colour index or a #rrggbb hex.
func validColor(v string) bool {
	_, ok := colorSeq(v, 38)
	return ok
}

// bgSeq is the escape that paints v as a background. It is kept as a raw
// string, not a lipgloss style, because a highlighted line has to re-apply
// it after every reset inside the text it covers.
func bgSeq(v string) string {
	seq, _ := colorSeq(v, 48)
	return seq
}

// colorSeq renders v as an SGR sequence, with base 38 for a foreground and
// 48 for a background. It reads "0".."255" and "#rrggbb".
func colorSeq(v string, base int) (string, bool) {
	if n, err := strconv.Atoi(v); err == nil {
		if n < 0 || n > 255 {
			return "", false
		}
		return fmt.Sprintf("\x1b[%d;5;%dm", base, n), true
	}
	hex, ok := strings.CutPrefix(v, "#")
	if !ok || len(hex) != 6 {
		return "", false
	}
	n, err := strconv.ParseUint(hex, 16, 32)
	if err != nil {
		return "", false
	}
	return fmt.Sprintf("\x1b[%d;2;%d;%d;%dm", base, n>>16&0xff, n>>8&0xff, n&0xff), true
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
