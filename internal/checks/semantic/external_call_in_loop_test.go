package semantic

import "testing"

func TestExternalCallInLoopConservativeCases(t *testing.T) {
	assertCheckDiagnostics(
		t,
		"external-call-in-loop",
		`package sample

import (
	"database/sql"
	"net/http"
)

func check(db *sql.DB, client *http.Client, request *http.Request, ids []int) {
	for range ids {
		db.Query("SELECT value FROM records")
		client.Do(request)
	}

	db.Exec("DELETE FROM records")
	for range ids {
		_ = func() {
			db.Query("SELECT nested FROM records")
		}
		go db.Exec("UPDATE records SET seen = TRUE")
	}
}
`,
		2,
		"inside a loop",
	)
}
