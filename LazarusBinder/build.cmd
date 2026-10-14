@echo off
cd /d "%~dp0"
powershell.exe -NoProfile -File scripts\build.ps1
if errorlevel 1 (
  echo Build failed. Read the error above.
  pause
  exit /b 1
)
echo Ready: dist\LazarusBinder.exe
pause
