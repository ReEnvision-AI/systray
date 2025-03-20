@echo off

:: Initialize the Podman machine
echo Initializing Podman machine...
podman machine init

:: Start the Podman machine
echo Starting Podman machine...
podman machine start

for /f "tokens=1* delims==" %%a in ('wmic path win32_VideoController get Name /value ^| findstr /v "^$"') do (
    set "GPUName=%%b"
    REM echo DEBUG: GPUName = "%GPUName%"
    echo "%GPUName%" | findstr /i "NVIDIA" >nul && goto nvidia
    echo "%GPUName%" | findstr /i "AMD" >nul && goto amd
    echo "%GPUName%" | findstr /i "ATI" >nul && goto amd
    echo "%GPUName%" | findstr /i "Intel" >nul && goto intel
)


echo Other or unknown GPU.
goto end

:nvidia
echo NVIDIA GPU detected.
:: Run the NVIDIA CDI generate command on the Podman machine
echo Running NVIDIA CDI generate command on the Podman machine...
podman machine ssh "sudo nvidia-ctk cdi generate --output=/etc/cdi/nvidia.yaml"
goto end

:amd
echo AMD GPU detected.
:: REM Add AMD-specific commands here
echo AMD GPU support will be added in the future...
goto end

:intel
echo Intel GPU detected.
REM Add Intel-specific commands here
echo Intel GPU support will be added in the future
goto end

:end


echo Completed all tasks.

