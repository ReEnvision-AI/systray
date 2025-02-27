# Initialize the Podman machine
Write-Output "Initializing Podman machine..."
podman machine init

# Start the Podman machine
Write-Output "Starting Podman machine..."
podman machine start

# Run the NVIDIA CDI generate command on the Podman machine
Write-Output "Running NVIDIA CDI generate command on the Podman machine..."
podman machine ssh "sudo nvidia-ctk cdi generate --output=/etc/cdi/nvidia.yaml && nvidia-ctk cdi list"

Write-Output "Completed all tasks."
