@echo off
rem LOCAL DEVELOPMENT ONLY. Runs the same checks as CI against the local stack:
rem Go vet + tests (race) for both services, then the client tests, typecheck and build.
rem The Go integration tests create and drop their own databases (thanh_test_*);
rem they never touch the thanh_dev data. Start the stack first: start-local.bat
setlocal
cd /d "%~dp0\..\.."
set TEST_DB=postgres://thanh_dev:DEV_ONLY_change_me_before_any_public_deployment@postgres:5432/thanh_dev?sslmode=disable

set PG_ID=
for /f %%i in ('docker compose -f deploy\compose.local.yml ps -q --status running postgres 2^>nul') do set PG_ID=%%i
if not defined PG_ID (
  echo [ERROR] PostgreSQL local chua chay. Hay chay deploy\windows\start-local.bat truoc.
  exit /b 1
)

echo [Project Thanh] Go: game-server + transaction-service (vet, test -race)...
docker run --rm --network deploy_default -e TEST_DATABASE_URL=%TEST_DB% -v "%CD%:/src" -v thanh-gomod:/go/pkg/mod -w /src golang:1.27 sh -c "cd game-server && test -z \"$(gofmt -l .)\" && go vet ./... && go test -race -count=1 ./... && cd ../transaction-service && test -z \"$(gofmt -l .)\" && go vet ./... && go test -race -count=1 -p 1 ./..."
if errorlevel 1 (
  echo [FAIL] Go tests
  exit /b 1
)

echo [Project Thanh] Client: test, typecheck, build...
pushd client
call npm ci || (popd & echo [FAIL] npm ci & exit /b 1)
call npm test || (popd & echo [FAIL] client tests & exit /b 1)
call npm run typecheck || (popd & echo [FAIL] typecheck & exit /b 1)
call npm run build || (popd & echo [FAIL] client build & exit /b 1)
popd

echo.
echo [PASS] Tat ca kiem thu tu dong. Kiem tra Admin end-to-end: node tools\dev\admin-e2e.mjs
endlocal
