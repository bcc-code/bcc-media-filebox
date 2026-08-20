package db

// SQLiteDSN returns the connection settings shared by production and tests.
// Foreign keys are per-connection in SQLite, so they must be enabled in the
// DSN rather than once with PRAGMA on an arbitrary pooled connection.
func SQLiteDSN(path string) string {
	return path + "?_journal_mode=WAL&_busy_timeout=5000&_pragma=foreign_keys(1)"
}
