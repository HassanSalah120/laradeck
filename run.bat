@echo off
setlocal
cd /d "%~dp0build\bin"
if exist "laradeck.exe" (
    start "" "laradeck.exe"
    echo LaraDeck launched successfully.
    exit /b 0
)
if exist "herd-workbench.exe" (
    start "" "herd-workbench.exe"
    echo LaraDeck launched successfully.
    exit /b 0
)
echo Error: Neither laradeck.exe nor herd-workbench.exe found in build\bin
pause
exit /b 1
