@echo off

echo Update WSL
wsl --update

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
echo NVIDIA GPU detected
:: Install NVIDIA Container Toolkit within Podman
echo Installing NVIDIA Container Toolkit...
podman machine ssh "sudo curl -s -L https://nvidia.github.io/libnvidia-container/stable/rpm/nvidia-container-toolkit.repo | sudo tee /etc/yum.repos.d/nvidia-container-toolkit.repo && sudo yum install -y nvidia-container-toolkit && sudo nvidia-ctk cdi generate --output=/etc/cdi/nvidia.yaml && nvidia-ctk cdi list"

goto end

:amd
echo AMD GPU detected
:: Add AMD-specific commands here
echo AMD GPU support will be added in the future...
goto end

:intel
echo Intel GPU detected
:: Add Intel-specific commands here
echo Intel GPU support will be added in the future
goto end

:end


echo Completed all tasks.

