// Package sqlconsole runs read-only analytical SQL against a Strikeye database.
// It is the escape hatch for questions the UI and exports do not cover: custom
// joins, ad hoc filters, and feeds into other tools.
package sqlconsole

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// Result is a materialised query result.
type Result struct {
	Columns   []string `json:"columns"`
	Rows      [][]any  `json:"rows"`
	Truncated bool     `json:"truncated"`
}

// forbidden lists statement keywords that can mutate state or reach outside the
// database file. Validation is belt-and-braces: the connection is also opened in
// read-only mode with query_only enabled.
var forbidden = []string{
	"insert", "update", "delete", "replace", "drop", "alter", "create",
	"attach", "detach", "pragma", "vacuum", "reindex", "trigger", "begin", "commit", "rollback",
}

// Validate checks that query is a single read-only statement.
func Validate(query string) error {
	clean := stripLiteralsAndComments(query)
	trimmed := strings.TrimSpace(clean)
	trimmed = strings.TrimSuffix(trimmed, ";")
	if trimmed == "" {
		return fmt.Errorf("query is empty")
	}
	if strings.Contains(trimmed, ";") {
		return fmt.Errorf("only one statement is allowed")
	}
	fields := strings.Fields(strings.ToLower(trimmed))
	if len(fields) == 0 {
		return fmt.Errorf("query is empty")
	}
	switch fields[0] {
	case "select", "with", "explain":
	default:
		return fmt.Errorf("only SELECT, WITH and EXPLAIN queries are allowed")
	}
	for _, word := range forbidden {
		if containsWord(fields, word) {
			return fmt.Errorf("keyword %q is not allowed in a read-only query", strings.ToUpper(word))
		}
	}
	return nil
}

// Run executes query with a row cap. When limit > 0 and the statement has no
// LIMIT of its own, the query is wrapped so the cap applies to the result set.
func Run(ctx context.Context, db *sql.DB, query string, limit int) (Result, error) {
	if err := Validate(query); err != nil {
		return Result{}, err
	}
	stmt := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(query), ";"))
	if limit > 0 {
		stmt = fmt.Sprintf("SELECT * FROM (%s) LIMIT %d", stmt, limit+1)
	}

	rows, err := db.QueryContext(ctx, stmt)
	if err != nil {
		return Result{}, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return Result{}, err
	}
	res := Result{Columns: cols}
	for rows.Next() {
		raw := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range raw {
			ptrs[i] = &raw[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return Result{}, err
		}
		for i, v := range raw {
			if b, ok := v.([]byte); ok {
				raw[i] = string(b)
			}
		}
		res.Rows = append(res.Rows, raw)
	}
	if err := rows.Err(); err != nil {
		return Result{}, err
	}
	if limit > 0 && len(res.Rows) > limit {
		res.Rows = res.Rows[:limit]
		res.Truncated = true
	}
	return res, nil
}

// stripLiteralsAndComments removes quoted strings and SQL comments so keyword
// checks cannot be fooled by or trip over their contents.
func stripLiteralsAndComments(query string) string {
	var b strings.Builder
	runes := []rune(query)
	for i := 0; i < len(runes); i++ {
		switch {
		case runes[i] == '\'' || runes[i] == '"' || runes[i] == '`':
			quote := runes[i]
			i++
			for i < len(runes) {
				if runes[i] == quote {
					if i+1 < len(runes) && runes[i+1] == quote {
						i += 2
						continue
					}
					break
				}
				i++
			}
			b.WriteRune(' ')
		case runes[i] == '-' && i+1 < len(runes) && runes[i+1] == '-':
			for i < len(runes) && runes[i] != '\n' {
				i++
			}
			b.WriteRune('\n')
		case runes[i] == '/' && i+1 < len(runes) && runes[i+1] == '*':
			i += 2
			for i+1 < len(runes) && !(runes[i] == '*' && runes[i+1] == '/') {
				i++
			}
			i++
			b.WriteRune(' ')
		default:
			b.WriteRune(runes[i])
		}
	}
	return b.String()
}

// containsWord reports whether the identifier appears as its own token.
func containsWord(fields []string, word string) bool {
	for _, f := range fields {
		if f == word {
			return true
		}
		if strings.HasPrefix(f, word+"(") || strings.HasPrefix(f, word+" ") {
			return true
		}
	}
	return false
}
