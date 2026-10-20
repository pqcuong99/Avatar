@echo off
setlocal
cd /d "%~dp0\..\.."
where docker >nul 2>nul
if errorlevel 1 (
  echo [ERROR] Docker CLI khong co san. Hay cai Docker Desktop tren may Windows ca nhan.
  pause
  exit /b 1
)
docker compose version >nul 2>nul
if errorlevel 1 (
  echo [ERROR] Docker Compose khong kha dung.
  pause
  exit /b 1
)
echo [Project Thanh] Dang khoi dong moi truong local...
docker compose -f deploy\compose.local.yml up -d --build
if errorlevel 1 (
  echo [ERROR] Khong the khoi dong Docker Compose.
  pause
  exit /b 1
)
echo.
echo [OK] Client demo: http://localhost:5173
echo [OK] Game health: http://localhost:8081/healthz
echo [OK] Transaction health: http://localhost:8082/healthz
echo [NOTE] Chi de phat trien local, KHONG chay tren VPS Windows Server 2012 R2.
pause
