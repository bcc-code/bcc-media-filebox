# FileBox

## Package Manager
- Use **pnpm** (not npm or yarn) for all frontend dependency management.

## Code Generation
- Run `make generate` after modifying SQL queries or schema.
- SQLc is used for Go database query generation.

## Migrations
- Use **goose** for database migrations. Migrations are embedded in the Go binary.
- **Never modify committed migrations.** Always create a new migration file.
- Migrations run automatically on startup.
- Migration files live in `internal/db/migrations/`.
