package semantic

import "testing"

func TestUnclosedSQLResourceTracksRowsAndStatements(t *testing.T) {
	reports := runResearchCorrectnessCheck(
		t,
		unclosedSQLResourceCheck{},
		`package fixture

import "database/sql"

func badRows(database *sql.DB) error {
	rows, err := database.Query("select 1")
	if err != nil { return err }
	_ = rows
	return nil
}

func goodRows(database *sql.DB) error {
	rows, err := database.Query("select 1")
	if err != nil { return err }
	defer rows.Close()
	return nil
}

func goodRowsDeferredWrapper(database *sql.DB) error {
	rows, err := database.Query("select 1")
	if err != nil { return err }
	defer func() { _ = rows.Close() }()
	return nil
}

func conditionalRowsClose(database *sql.DB, closeRows bool) error {
	rows, err := database.Query("select 1")
	if err != nil { return err }
	if closeRows {
		defer rows.Close()
	}
	return nil
}

func badStatement(database *sql.DB) error {
	statement, err := database.Prepare("select 1")
	if err != nil { return err }
	_ = statement
	return nil
}

func goodStatement(database *sql.DB) error {
	statement, err := database.Prepare("select 1")
	if err != nil { return err }
	return statement.Close()
}

func transferred(database *sql.DB) (*sql.Rows, error) {
	rows, err := database.Query("select 1")
	if err != nil { return nil, err }
	return rows, nil
}
`,
	)
	assertResearchReportCount(t, reports, 3)
	assertResearchMessagesContain(t, reports, "sql.")
}

func TestUnclosedSQLResourceTracksAliasGenerations(t *testing.T) {
	source := `package fixture

import "database/sql"

func rowsAlias(database *sql.DB) {
	rows, _ := database.Query("rows-first")
	oldRows := rows
	rows, _ = database.Query("rows-second")
	defer oldRows.Close()
}

func statementAlias(database *sql.DB) {
	statement, _ := database.Prepare("statement-first")
	oldStatement := statement
	statement, _ = database.Prepare("statement-second")
	defer oldStatement.Close()
}

func bothClosed(database *sql.DB) {
	rows, _ := database.Query("closed-first")
	oldRows := rows
	rows, _ = database.Query("closed-second")
	defer oldRows.Close()
	defer rows.Close()
}

func pathDependent(database *sql.DB, useNew bool) {
	rows, _ := database.Query("path-first")
	alias := rows
	rows, _ = database.Query("path-second")
	if useNew {
		alias = rows
	}
	defer alias.Close()
}
`
	reports := runResearchCorrectnessCheck(t, unclosedSQLResourceCheck{}, source)
	assertResearchReportCount(t, reports, 4)
	assertResearchReportNeedles(
		t,
		reports,
		source,
		"database.Query(\"rows-second\")",
		"database.Prepare(\"statement-second\")",
		"database.Query(\"path-first\")",
		"database.Query(\"path-second\")",
	)
}

func TestUnclosedSQLResourceTreatsNamedResultsAsTransfers(t *testing.T) {
	reports := runResearchCorrectnessCheck(
		t,
		unclosedSQLResourceCheck{},
		`package fixture

import "database/sql"

func namedRows(database *sql.DB) (rows *sql.Rows, err error) {
	rows, err = database.Query("rows")
	return
}

func namedStatement(database *sql.DB) (statement *sql.Stmt, err error) {
	statement, err = database.Prepare("statement")
	return
}
`,
	)
	assertResearchReportCount(t, reports, 0)
}
