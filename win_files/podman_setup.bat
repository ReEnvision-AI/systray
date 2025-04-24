@echo off
PowerShell -WindowStyle Hidden -ExecutionPolicy Bypass -nologo -File "%PROGRAMFILES%\ReEnvision AI\post_wsl_install.ps1" >> "%TEMP%\PodmanSetupLog.txt" 2>&1