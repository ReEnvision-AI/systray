@echo off

:: Initialize the Podman machine
echo Initializing Podman machine...
podman machine init

:: Start the Podman machine
echo Starting Podman machine...
podman machine start

:: Run the NVIDIA CDI generate command on the Podman machine
echo Running NVIDIA CDI generate command on the Podman machine...
podman machine ssh "sudo nvidia-ctk cdi generate --output=/etc/cdi/nvidia.yaml"

echo Completed all tasks.
pause
