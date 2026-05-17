# Changelog

## [Unreleased]

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

### Changed
- **sql.go**: Replaced raw string context key `"tx"` with typed constant `txKey` of unexported type `ctxKey` to prevent context key collisions
- **sql.go**: Renamed `SqlTx` → `SQLTx` and `NewSqlTx` → `NewSQLTx` per Go acronym naming convention
- **All files**: Replaced `github.com/pkg/errors` (deprecated) with standard library `fmt.Errorf` + `errors`
- **uow.go**: Rollback error in double-failure path now uses double `%w` wrapping, making both the fn error and rollback error accessible via `errors.Is`

### Fixed
- **mongo.go**: Session leak in `Ctx` — `sess.EndSession(ctx)` is now called when `StartTransaction` fails
- **mock.go**: Added missing package comment, renamed unused `ctx` parameters to `_`
- **uow_test.go**: Renamed unused `ctx` parameters to `_` in mock runners and callbacks

### Removed
- `github.com/pkg/errors` dependency
