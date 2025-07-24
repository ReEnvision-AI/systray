wsl --set-default-version 2

wsl --update

Write-Output "Initializing Podman machine..."
podman machine init

Write-Output "Starting Podman machine..."
podman machine start

Write-Output "Configuring Podman machine..."
podman machine ssh "sudo curl -s -L https://nvidia.github.io/libnvidia-container/stable/rpm/nvidia-container-toolkit.repo | sudo tee /etc/yum.repos.d/nvidia-container-toolkit.repo && sudo yum install -y nvidia-container-toolkit && sudo nvidia-ctk cdi generate --output=/etc/cdi/nvidia.yaml && nvidia-ctk cdi list"