package analyze_cases

import "database/sql"

func iterateRowsWithErrorCheck(rows *sql.Rows) error {
	for rows.Next() {
	}
	return rows.Err()
}
