# ReEnvision AI Installation Directory

This directory contains the necessary files to install and set up ReEnvision AI.

## Files in this Directory

* **`installer.iss`**: This is the Inno Setup script that defines the installation process for ReEnvision AI.
* **`ReEnvisionAI.exe`**: The main executable file for the ReEnvision AI application.
* **`reai.ico`**: The icon file used for the ReEnvision AI application and installer.
* **`podman-5.4.1-setup.exe`**: The installer for Podman, a containerization engine.
* **`podman_setup.ps1`**: A PowerShell script that configures Podman for ReEnvision AI, including updating WSL, initializing and starting the Podman machine, and setting up NVIDIA GPU support if detected.
* **`podman_setup.bat`**: A batch script that executes the `podman_setup.ps1` script.

## Installation Instructions

To install ReEnvision AI, please run the `ReEnvision AI Setup.exe` file that will be generated when the `installer.iss` script is compiled using Inno Setup.

The installation process will perform the following steps:

1.  **Install ReEnvision AI**: The main application files will be installed to `C:\Program Files\ReEnvision AI` (or a similar location depending on your system).
2.  **Install Podman**: The Podman installer will be run in the background to install the containerization engine required by ReEnvision AI.
3.  **Enable Windows Subsystem for Linux (WSL) and Virtual Machine Platform (VMP)**: If these features are not already enabled on your system, the installer will attempt to enable them. This may require a system reboot.
4.  **Configure Podman**: The `podman_setup.ps1` script will be executed to:
    * Update WSL to the latest version.
    * Initialize and start the Podman machine.
    * Detect your system's GPU.
    * If an NVIDIA GPU is detected, it will attempt to configure the NVIDIA Container Toolkit to enable GPU support within Podman containers.
5.  **Set up Firewall Rule**: A firewall rule will be added to allow network communication for ReEnvision AI.
6.  **Create Shortcuts**: Shortcuts to ReEnvision AI will be created in your Start Menu, on your Desktop (optional), and in other relevant locations.

## Prerequisites

Before running the installer, ensure that your system meets the following requirements:

* **Operating System**: Windows 10 or later is recommended.
* **CPU Virtualization Enabled**: CPU virtualization must be enabled in your computer's BIOS settings. The installer will check for this and prompt you if it is not enabled.
* **Administrator Privileges**: You will need administrator privileges to run the installer and enable system features like WSL and VMP.

## Post-Installation

After the installation is complete, ReEnvision AI should be ready to use. The `podman_setup.bat` script will also run the Podman configuration script on the first user logon after installation to ensure Podman is properly set up.

## Support

For any issues or questions regarding the installation or usage of ReEnvision AI, please visit our website at [https://reenvision.ai](https://reenvision.ai).

Copyright © 2025 ReEnvision AI