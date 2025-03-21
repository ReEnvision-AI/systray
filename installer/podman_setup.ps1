# Update WSL
Write-Output "Updating Windows Subsystem for Linux..."
wsl --update

# Initialize the Podman machine
Write-Output "Initializing Podman machine..."
podman machine init

# Start the Podman machine
Write-Output "Starting Podman machine..."
podman machine start

# Retrieve all video controllers
$videoControllers = Get-CimInstance -ClassName Win32_VideoController

# Optional: Display information about each GPU
foreach ($controller in $videoControllers) {
    $compat = $controller.AdapterCompatibility
    $name = $controller.Name
    if ($compat -like "*NVIDIA*") {
        Write-Output "NVIDIA GPU detected: $name"
    } elseif ($compat -like "*Advanced Micro Devices*" -or $compat -like "*ATI Technologies*") {
        Write-Output "AMD GPU detected: $name"
    } elseif ($compat -like "*Intel*") {
        Write-Output "Intel GPU detected: $name"
    } else {
        Write-Output "Unknown GPU: $name"
    }
}

# Check if any NVIDIA GPU is present and run commands
if ($videoControllers | Where-Object { $_.AdapterCompatibility -like "*NVIDIA*" }) {
    Write-Output "NVIDIA GPU found..."
    # Run the NVIDIA CDI generate command on the Podman machine
    Write-Output "Running NVIDIA CDI generate command on the Podman machine..."
    podman machine ssh "sudo curl -s -L https://nvidia.github.io/libnvidia-container/stable/rpm/nvidia-container-toolkit.repo | sudo tee /etc/yum.repos.d/nvidia-container-toolkit.repo && sudo yum install --disableplugin subscription-manager -y nvidia-container-toolkit && sudo nvidia-ctk cdi generate --output=/etc/cdi/nvidia.yaml"
    
}



Write-Output "Completed all tasks."
Pause