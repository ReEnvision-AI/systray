# ReEnvision AI

ReEnvision AI is a system tray application for Windows that leverages the Petals framework to run large language models. It utilizes Podman for containerization to manage the Petals server.

## Features

* **System Tray Application:** Runs silently in the system tray, providing easy access to start and stop the service.
* **Petals Integration:** Runs the `meta-llama/Llama-3.3-70B-Instruct` model using the Petals distributed inference framework.
* **Containerized with Podman:** Uses Podman to manage the Petals server within a container, ensuring a consistent and isolated environment.
* **GPU Acceleration (Optional):** Automatically detects NVIDIA GPUs and attempts to configure Podman to utilize them for accelerated inference.
* **Configurable Port:** Allows you to configure the port the Petals server listens on via the Windows Registry.
* **Logging:** Logs application activity and Petals server output to a file for debugging and monitoring.
* **Single Instance:** Ensures only one instance of the application runs at a time.

## Prerequisites

Before installing and running ReEnvision AI, ensure you have the following prerequisites installed and configured:

* **Windows Subsystem for Linux (WSL):** ReEnvision AI relies on Podman running within WSL. Please ensure you have WSL installed and a Linux distribution (like Ubuntu) set up. You can find instructions on how to install WSL on the official Microsoft documentation.
* **Podman:** Podman needs to be installed within your WSL environment. You can typically install it using your Linux distribution's package manager (e.g., `sudo apt-get install podman` on Ubuntu).
* **NVIDIA Drivers (Optional for GPU Acceleration):** If you intend to use GPU acceleration, ensure you have the latest NVIDIA drivers installed on your Windows system.
* **CPU Virtualization Enabled:** Make sure that CPU virtualization is enabled in your computer's BIOS settings.
* **Go Programming Language:** You will need Go (version 1.18 or later is recommended) installed on your development machine to build the application from source. You can download and install Go from the official Go website: [https://go.dev/dl/](https://go.dev/dl/)

## Installation

This application is designed to run as a compiled executable on Windows. You can either download a pre-built executable (if available) or build it from the source code.

### Building from Source

If you wish to build the ReEnvision AI application from its source code, follow these steps:

1.  **Clone the Repository:** If you have access to the source code repository, clone it to your local machine using Git or your preferred method.
2.  **Navigate to the Project Directory:** Open your terminal or command prompt and navigate to the root directory of the ReEnvision AI project.
3.  **Build the Application:** Run the following command to build the executable:

    ```bash
    go build -ldflags "-H=windowsgui" -o scripts/installer/ReEnvisionAI.exe .\cmd\reenvisionai\
    ```

    * `go build`: This is the command to build Go programs.
    * `-ldflags "-H=windowsgui"`: This linker flag is specific to Windows and tells the linker to create a GUI application, which will run without a console window.
    * `-o scripts/installer/ReEnvisionAI.exe`: This specifies the output file name and location. The executable will be named `ReEnvisionAI.exe` and placed in a subdirectory named `installer` under `scripts`. 

4.  **Locate the Executable:** Once the build process is complete, the `ReEnvisionAI.exe` file will be located in the `scripts\installer` directory within your project.

### Running the Executable

Once you have the `ReEnvisionAI.exe` file (either downloaded or built from source), you can run it by double-clicking the file.

## Usage

Once the application starts, it will appear as an icon in your system tray.

* **Start:** Right-click the system tray icon and select "Start". This will:
    * Ensure Podman is running within WSL.
    * Attempt to configure NVIDIA GPU support for Podman if an NVIDIA GPU is detected.
    * Start a Podman container named "ReEnvisionAI" running the Petals server with the specified model and parameters.
* **Stop:** To stop the ReEnvision AI service, right-click the system tray icon and select "Stop". This will stop and remove the Podman container.
* **Quit:** To completely exit the application, right-click the system tray icon and select "Quit". This will also attempt to stop the Podman container before exiting.

## Configuration

### Port Configuration

You can configure the port that the Petals server listens on by modifying a value in the Windows Registry.

1.  Open the Registry Editor by searching for "regedit" in the Windows search bar and running the application.
2.  Navigate to the following key: `HKEY_LOCAL_MACHINE\SOFTWARE\ReEnvisionAI\ReEnvisionAI`
3.  If the `ReEnvisionAI` key or the `Port` value does not exist, you may need to create them:
    * Right-click on `SOFTWARE`, select "New" -> "Key", and name it `ReEnvisionAI`.
    * Right-click on the newly created `ReEnvisionAI` key, select "New" -> "Key", and name it `ReEnvisionAI`.
    * Right-click on the inner `ReEnvisionAI` key, select "New" -> "DWORD (32-bit) Value", and name it `Port`.
4.  Double-click the `Port` value and set its value data to the desired port number (in decimal). The default port is `31330`.
5.  Restart the ReEnvision AI application for the changes to take effect.

## Logging

ReEnvision AI logs its activity and the output of the Petals server to a file named `log.txt`. This file can be found in the following directory:

`%APPDATA%\ReEnvisionAI`

You can access this directory by typing `%APPDATA%\ReEnvisionAI` in the Windows File Explorer address bar.

## Important Notes

* The first time you start ReEnvision AI, Podman may need to download the `ghcr.io/reenvision-ai/petals:latest` container image, which can take some time depending on your internet connection.
* Running large language models can be resource-intensive. Ensure your system has sufficient RAM and processing power.
* GPU acceleration requires a compatible NVIDIA GPU and properly installed drivers. The application will attempt to configure this automatically.
* The specific model being used is `meta-llama/Llama-3.3-70B-Instruct`. Since it is a gated model, a HuggingFace token might be needed.
* The application uses a specific initial peer for the Petals network: `/dns4/sociallyshaped.net/tcp/8788/p2p/QmTUpY86VSyvwvBN8oc9W3JztLaxyabT6b17gnXxdfx5HL`.

## Troubleshooting

If you encounter any issues, please check the `log.txt` file for error messages. Ensure that WSL and Podman are correctly installed and configured. If you are trying to use GPU acceleration, verify that your NVIDIA drivers are up to date.

For further assistance or to report issues, please refer to the project's repository or contact the developers.