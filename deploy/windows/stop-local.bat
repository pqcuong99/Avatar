@echo off
setlocal
cd /d "%~dp0\..\.."
docker compose -f deploy\compose.local.yml down
if errorlevel 1 exit /b 1
echo [OK] Da dung cac container local. Du lieu database duoc giu trong Docker volume.
pause
