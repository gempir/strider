package semantic

import "testing"

func TestUncheckedRowsErrorReportsMissingIterationCheck(t *testing.T) {
	root := analysisModule(
		t,
		`package sample

import "database/sql"

func bad(rows *sql.Rows) {
	for rows.Next() {}
}

func good(rows *sql.Rows) error {
	for rows.Next() {}
	return rows.Err()
}
`,
	)
	registry, err := newRegistry([]string{
		"unchecked-rows-error",
	})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := Run([]string{
		root,
	}, registry)
	if err != nil {
		t.Fatal(err)
	}
	assertDiagnosticGolden(t, diagnostics)
}
