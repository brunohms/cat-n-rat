package purchase

import (
	"strconv"
	"strings"
)

// whereBuilder collects SQL conditions and their arguments, numbering the placeholders.
type whereBuilder struct {
	conditions []string
	args       []any
}

// addIf adds the condition when ok is true; every "?" in it refers to the same argument.
func (w *whereBuilder) addIf(ok bool, condition string, arg any) {
	if !ok {
		return
	}
	w.args = append(w.args, arg)
	w.conditions = append(w.conditions, strings.ReplaceAll(condition, "?", "$"+strconv.Itoa(len(w.args))))
}

func (w *whereBuilder) sql() string {
	if len(w.conditions) == 0 {
		return ""
	}
	return "WHERE " + strings.Join(w.conditions, " AND ")
}

// containsPattern builds an ILIKE pattern that matches the text anywhere, escaping LIKE wildcards.
func containsPattern(text string) string {
	escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(text)
	return "%" + escaped + "%"
}

func sortDirection(desc bool) string {
	if desc {
		return "DESC"
	}
	return "ASC"
}
