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
OutputBaseFilename=ReEnvision AI Setup
Compression=lzma
SolidCompression=yes
PrivilegesRequired=admin
ArchitecturesInstallIn64BitMode=x64compatible
SetupIconFile=reai.ico
SetupLogging=yes

[Icons]
Name: "{group}\ReEnvision AI"; Filename: "{app}\ReEnvisionAI.exe"; WorkingDir: "{app}"
Name: "{commondesktop}\ReEnvision AI"; Filename: "{app}\ReEnvisionAI.exe"; WorkingDir: "{app}"
Name: "{commonprograms}\ReEnvision AI"; Filename: "{app}\ReEnvisionAI.exe"; WorkingDir: "{app}"
Name: "{commonstartmenu}\ReEnvision AI"; Filename: "{app}\ReEnvisionAI.exe"; WorkingDir: "{app}"
Name: "{commonstartup}\ReEnvision AI"; Filename: "{app}\ReEnvisionAI.exe"; WorkingDir: "{app}"

[Files]
Source: "ReEnvisionAI.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "podman-5.4.1-setup.exe"; DestDir: "{tmp}"; Flags: deleteafterinstall
Source: "podman_setup.bat"; DestDir: "{app}"; Flags: ignoreversion

[Run]
Filename: "{tmp}\podman-5.4.1-setup.exe"; Parameters: "/quiet"; Flags: shellexec  waituntilterminated; StatusMsg: "Installing Podman, please wait..."
Filename: "netsh"; Parameters: "advfirewall firewall delete rule name=""ReEnvision AI"""; Flags:  waituntilterminated
Filename: "netsh"; Parameters: "advfirewall firewall add rule name=""ReEnvision AI"" dir=in action=allow protocol=TCP localport=31330"; Flags:  waituntilterminated; StatusMsg: "Setting up firewall rule, please wait..."

Filename: "powershell"; Parameters: "-NoProfile -ExecutionPolicy Bypass -Command ""wsl --install --no-distribution"""; Check: NeedsWSLInstall; Flags: waituntilterminated; StatusMsg: "Setting up Windows Subsystem for Linux, please wait..."


[Registry]
Root: HKLM; Subkey: "SOFTWARE\Microsoft\Windows\CurrentVersion\RunOnce"; ValueType: string; ValueName: "ReEnvisionAI_Setup"; ValueData: "cmd.exe /c ""{app}\podman_setup.bat"""; Flags: uninsdeletevalue


[UninstallRun]
Filename: "netsh"; Parameters: "advfirewall firewall delete rule name=""ReEnvision AI"""; Flags: waituntilterminated; RunOnceId: "DeleteReEnvisionAIFirewallRule"

[Code]
var
  WSLInstalled: Boolean;

function CheckVirtualizationEnabled(): Boolean;
var
  TempFile: String;
  Lines: TArrayOfString;
  I: Integer;
  Line: String;
begin
  TempFile := ExpandConstant('{tmp}\systeminfo.txt');
  if Exec('cmd.exe', '/c systeminfo > "' + TempFile + '"', '', SW_HIDE, ewWaitUntilTerminated, I) then
  begin
    if LoadStringsFromFile(TempFile, Lines) then
    begin
      for I := 0 to GetArrayLength(Lines) - 1 do
      begin
        Line := Trim(Lines[I]);
        if (Pos('A hypervisor has been detected', Line) > 0) or
           (Pos('Virtualization Enabled In Firmware: Yes', Line) > 0) then
        begin
          Result := True;
          Exit;
        end
        else if Pos('Virtualization Enabled In Firmware: No', Line) > 0 then
        begin
          Result := False;
          Exit;
        end;
      end;
    end;
  end;
  Result := False; // Default to false if check fails
end;

function IsWSLEnabled(): Boolean;
var
  ResultCode: Integer;
begin
  if Exec('powershell.exe', '-Command "$feature = Get-WindowsOptionalFeature -Online -FeatureName Microsoft-Windows-Subsystem-Linux; if ($feature.State -eq ''Enabled'') { exit 0 } else { exit 1 }"', '', SW_HIDE, ewWaitUntilTerminated, ResultCode) then
  begin
    Result := (ResultCode = 0);
  end
  else
  begin
    Result := False;
  end;
end;

function NeedsWSLInstall(): Boolean;
begin
  if not IsWSLEnabled then
  begin
    WSLInstalled := True;
    Result := True;
  end
  else
  begin
    WSLInstalled := False;
    Result := False;
  end;
end;



function InitializeSetup(): Boolean;
begin
  if not CheckVirtualizationEnabled then
  begin
    MsgBox('CPU Virtualization is not enabled in the BIOS. Please consult your motherboard manual on how to enable it.', mbError, MB_OK)
    Result := False;
  end
  else
  begin
    Result := True;
  end;
end;

