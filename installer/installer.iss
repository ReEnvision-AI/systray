[Setup]
; Basic setup information
AppName=ReEnvision AI
AppVersion=1.0.0
AppPublisher=ReEnvision AI
AppPublisherURL=https://reenvision.ai
AppCopyright=2025
AppId={{6F22380A-0A5A-4705-A0ED-D1DBDF18484A}}
DefaultDirName={autopf}\ReEnvision AI
DefaultGroupName=ReEnvision AI
OutputBaseFilename=ReEnvisionAISetup
Compression=lzma
SolidCompression=yes
PrivilegesRequired=admin

[Icons]
Name: "{group}\ReEnvision AI"; Filename: "{app}\ReEnvisionAI.exe"; WorkingDir: "{app}"
Name: "{commondesktop}\ReEnvision AI"; Filename: "{app}\ReEnvisionAI.exe"; WorkingDir: "{app}"
Name: "{commonprograms}\ReEnvision AI"; Filename: "{app}\ReEnvisionAI.exe"; WorkingDir: "{app}"
Name: "{commonstartmenu}\ReEnvision AI"; Filename: "{app}\ReEnvisionAI.exe"; WorkingDir: "{app}"
Name: "{commonstartup}\ReEnvision AI"; Filename: "{app}\ReEnvisionAI.exe"; WorkingDir: "{app}"



[Files]
; Add MyApp.exe to the installation directory
Source: "ReEnvisionAI.exe"; DestDir: "{app}"; Flags: ignoreversion
; Include the Podman installer
Source: "podman-5.3.2-setup.exe"; DestDir: "{tmp}"; Flags: deleteafterinstall
; Include the Podman Setup Script
Source: "podman_setup.bat"; DestDir: "{tmp}"; Flags: deleteafterinstall

[Run]
; Silent installation of Podman after ensuring WSL is installed
Filename: "{tmp}\podman-5.3.2-setup.exe"; Parameters: "WSLCheckbox=1 /SILENT"; Flags: shellexec runhidden



Filename: "{tmp}\podman_setup.bat"; Flags: shellexec runhidden

[Code]
function IsWSLInstalled(): Boolean;
var
  ResultCode: Integer;
begin
  Result := Exec('powershell.exe', '-Command "wsl --list"', '', SW_HIDE, ewWaitUntilTerminated, ResultCode) and (ResultCode = 0);
end;

function EnableWSL(): Boolean;
var
  ResultCode: Integer;
begin
  Result := Exec('powershell.exe', '-Command "Enable-WindowsOptionalFeature -FeatureName Microsoft-Windows-Subsystem-Linux -Online -NoRestart"', '', SW_HIDE, ewWaitUntilTerminated, ResultCode) and (ResultCode = 0);
end;

function InstallWSL(): Boolean;
var
  ResultCode: Integer;
begin
  Result := Exec('powershell.exe', '-Command "wsl --install --no-distribution"', '', SW_HIDE, ewWaitUntilTerminated, ResultCode) and (ResultCode = 0);
end;

function InitializeSetup(): Boolean;
begin
  Result := True;

  if not IsWSLInstalled() then
  begin
    MsgBox('Windows Subsystem for Linux (WSL) is required. It will now be installed.', mbInformation, MB_OK);
    if not EnableWSL() then
    begin
      MsgBox('Failed to enable WSL. Please enable manually and try again.', mbError, MB_OK);
      Result := False;
      Exit;
    end;
    if not InstallWSL() then
    begin
      Result := False;
      MsgBox('Failed to install WSL. Please install it manually and try again.', mbError, MB_OK);
      Exit;
    end
    else
    begin
      MsgBox('WSL has been installed successfully. Please restart your computer before continuing.', mbInformation, MB_OK);
      Result := False; // Ensure the user restarts the system before proceeding
    end;
  end;
end;
