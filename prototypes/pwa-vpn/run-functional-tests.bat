@echo off
REM FUNC-COV-001: Functional suite gate
cd /d "%~dp0\.."
echo ============================================
echo   FUNCTIONAL TEST GATE - Indestructible
echo ============================================
echo.
echo [1/3] Go tests...
set GOMAXPROCS=1
go test ./src/src-vpn/... -count=1 -timeout 120s
if errorlevel 1 (echo GO FAIL & exit /b 1)
echo   Go: PASS
echo.
echo [2/3] Vitest...
call npx vitest run --reporter=verbose
if errorlevel 1 (echo VITEST FAIL & exit /b 1)
echo   Vitest: PASS
echo.
echo [3/3] Playwright...
call npx playwright test e2e/ --reporter=list --timeout 30000
if errorlevel 1 (echo PLAYWRIGHT FAIL & exit /b 1)
echo   Playwright: PASS
echo.
echo ============================================
echo   ALL GATES GREEN
echo ============================================
