package analyze_cases

import "database/sql"

func iterateRowsWithoutErrorCheck(rows *sql.Rows) {
	for rows.Next() {
	}
}
