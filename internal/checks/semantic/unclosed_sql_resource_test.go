package semantic

import "testing"

func TestUnclosedSQLResourceChecksDirectOwnership(t *testing.T) {
	reports := runResearchCorrectnessCheck(
		t,
		unclosedSQLResourceCheck{},
		`package fixture

import "database/sql"

func missing(database *sql.DB) error {
	rows, err := database.Query("select 1")
	if err != nil { return err }
	_ = rows
	return nil
}

func closed(database *sql.DB) error {
	rows, err := database.Query("select 1")
	if err != nil { return err }
	defer rows.Close()
	return nil
}

func transferred(database *sql.DB) (*sql.Rows, error) {
	rows, err := database.Query("select 1")
	if err != nil { return nil, err }
	return rows, nil
}

func replaced(database *sql.DB) {
	statement, _ := database.Prepare("first")
	statement, _ = database.Prepare("second")
	defer statement.Close()
}
`,
	)
	assertResearchReportCount(t, reports, 2)
	assertResearchMessagesContain(t, reports, "sql.")
}

func TestUnclosedSQLResourceLeavesAliasesToOwnershipAnalysis(t *testing.T) {
	reports := runResearchCorrectnessCheck(
		t,
		unclosedSQLResourceCheck{},
		`package fixture

import "database/sql"

func alias(database *sql.DB) error {
	rows, err := database.Query("select 1")
	if err != nil { return err }
	alias := rows
	defer alias.Close()
	return nil
}
`,
	)
	assertResearchReportCount(t, reports, 0)
}
