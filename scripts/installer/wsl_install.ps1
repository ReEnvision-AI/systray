$osInfo = Get-WmiObject -Class Win32_OperatingSystem
$osCaption = $osInfo.Caption

if ($osCaption -match "Windows 10") {
    Write-Host "Windows 10 detected"
    Enable-WindowsOptionalFeature -Online -FeatureName Microsoft-Windows-Subsystem-Linux -NoRestart
    Enable-WindowsOptionalFeature -Online -FeatureName VirtualMachinePlatform -NoRestart
} elseif ($osCaption -match "Windows 11") {
    Write-Host "Windows 11 detected "
    wsl --install --no-distribution
} else {
    Write-Host "Unsupported Windows version. Please use Windows 10 or Windows 11."
    exit
}

