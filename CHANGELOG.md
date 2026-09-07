# Changelog

## [0.9.0] - 2026-09-07

### Added
- **PostgreSQL integration tests**: the GitHub Actions workflow now starts a
  `postgres:16` service container and sets `POSTGRES_URI`, so the SQL runner
  is tested against a real PostgreSQL server (commit, rollback, and nested
  transactions). Tests use `github.com/jackc/pgx/v5/stdlib` as the driver.

## [0.8.0] - 2026-09-06

### Added
- **Mongo integration tests in CI**: the GitHub Actions workflow now starts a
  MongoDB service container and sets `MONGODB_URI`, so the Mongo transaction
  integration tests actually run instead of skipping.
- **Dependabot**: weekly updates for Go modules and GitHub Actions.
- **Godoc examples**: `ExampleUoW_Run_sql` (SQLite via the typed Executor) and
  `ExampleUoW_Run_mongo` (mock via the typed State accessor).
- **Benchmarks**: `BenchmarkRun`, `BenchmarkRun_Nested` (core), and
  `BenchmarkSqlTx_Commit` (SQL).

### Changed
- **CI test command**: now runs with `-race`.

## [0.7.0] - 2026-09-06

### Changed
- **Panic handling in `Run`**: if `fn` panics, the outermost `Run` now rolls
  back the transaction and re-panics, so the caller still observes the panic
  but no transaction is leaked. Previously a panic left the transaction open.

### Documented
- **Commit failure behavior**: if `Commit` fails, the transaction is left in an
  unknown state and `Rollback` is not attempted.

## [0.6.0] - 2026-09-06

### Added
- **`mock.Tx.State(ctx)` method**: a type-safe accessor returning the
  `*State` object. No type assertions needed.

## [0.5.0] - 2026-09-06

### Added
- **`mongo.Tx.Database(ctx)` method**: a type-safe accessor returning the
  `*mongo.Database` handle. Inside a transaction it returns the database bound
  to the active session; outside a transaction it returns the client's
  database. No type assertions needed.

## [0.4.0] - 2026-09-06

### Added
- **`sql.Executor` interface and `sql.Tx.Executor(ctx)` method**: a type-safe
  accessor returning the active SQL handle (`*sql.Tx` inside a transaction,
  `*sql.DB` outside). Both implement the `Executor` interface, so repository
  code can run the same statements in or out of a transaction without type
  assertions.

## [0.3.0] - 2026-09-06

### Breaking Changes
- **Package split into subpackages**: The library is now split into `uow` (core), `uow/sql`, `uow/mongo`, and `uow/mock`. Import paths changed:
  - `uow.NewSQLTx` → `uow/sql.NewTx`
  - `uow.NewMongoTx` → `uow/mongo.NewTx`
  - `uow.NewMockTx` → `uow/mock.NewTx`
  - `uow.State` → `uow/mock.State`
  - The `Runner` interface and `UoW` type remain in the core `uow` package.
- **Type renames**: `SQLTx` → `Tx`, `MongoTx` → `Tx`, `MockTx` → `Tx` (in their respective subpackages) to avoid stuttering names.
- **Dependency isolation**: SQL-only users no longer pull in the MongoDB driver; Mongo-only users no longer pull in SQL test dependencies.

### Added
- Nested transaction support: `Run` calls can be nested; inner calls reuse the outer transaction and their `Commit`/`Rollback` become no-ops. Inner errors propagate to the outermost `Run`, which rolls back the entire transaction.

## [0.2.1] - 2026-05-17

### Added
- `.golangci.yml` with standard linters (govet, errcheck, staticcheck, revive, etc.)
- GitHub Actions workflow: `golangci-lint` step on push/PR
- Comprehensive test suite:
  - Table-driven tests for `UoW.Run` covering all error paths (ctx error, fn error, double failure, commit error)
  - SQLite in-memory integration tests (`TestSqlTx_Commit`, `TestSqlTx_Rollback`, `TestSqlTx_GetReturnDB`)
  - MongoDB integration tests (skipped unless `MONGODB_URI` is set)
  - Context cancellation propagation test
  - Runnable example test (`ExampleUoW_Run`)
- `Makefile` with `test`, `lint`, `coverage`, `build`, `tidy`, `clean` targets
- README badges (Go version, license, CI status)
- README installation guide and changelog link
- README architecture section documenting `Runner` interface and `UoW` lifecycle
- README development guide with `make` commands
- `SQLTx` added to README implementations list

### Changed
- **sql.go**: Replaced raw string context key `"tx"` with typed constant `txKey` of unexported type `ctxKey` to prevent context key collisions
- **sql.go**: Renamed `SqlTx` → `SQLTx` and `NewSqlTx` → `NewSQLTx` per Go acronym naming convention
- **All files**: Replaced `github.com/pkg/errors` (deprecated) with standard library `fmt.Errorf` + `errors`
- **uow.go**: Rollback error in double-failure path now uses double `%w` wrapping, making both the fn error and rollback error accessible via `errors.Is`
- **README.md**: Fixed `NewSqlTx` → `NewSQLTx` in SQL example
- **README.md**: Enhanced contributing section with `make lint && make test` instructions

### Fixed
- **mongo.go**: Session leak in `Ctx` — `sess.EndSession(ctx)` is now called when `StartTransaction` fails
- **mock.go**: Added missing package comment, renamed unused `ctx` parameters to `_`
- **uow_test.go**: Renamed unused `ctx` parameters to `_` in mock runners and callbacks

### Removed
- `github.com/pkg/errors` dependency
