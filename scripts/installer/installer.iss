[Setup]
; Basic setup information
AppName=ReEnvision AI
AppVersion=1.0.2
AppVerName=ReEnvision AI
AppPublisher=ReEnvision AI, LLC
AppPublisherURL=https://reenvision.ai
AppCopyright=2025
AppId={{6F22380A-0A5A-4705-A0ED-D1DBDF18484A}}
DefaultDirName={autopf}\ReEnvision AI
DefaultGroupName=ReEnvision AI
OutputBaseFilename=ReEnvisionAISetup
Compression=lzma
SolidCompression=yes
PrivilegesRequired=admin
ArchitecturesInstallIn64BitMode=x64compatible
SetupIconFile=reai.ico
SetupLogging=yes
SignedUninstaller=yes
SetupMutex=Global\ReEnvisionSetupMutex
SignTool=MsSign $f
DirExistsWarning=no
UsedUserAreasWarning=no

[Icons]
Name: "{group}\ReEnvision AI"; Filename: "{app}\ReEnvisionAI.exe"; WorkingDir: "{app}"
Name: "{commondesktop}\ReEnvision AI"; Filename: "{app}\ReEnvisionAI.exe"; WorkingDir: "{app}"
Name: "{commonprograms}\ReEnvision AI"; Filename: "{app}\ReEnvisionAI.exe"; WorkingDir: "{app}"
Name: "{commonstartmenu}\ReEnvision AI"; Filename: "{app}\ReEnvisionAI.exe"; WorkingDir: "{app}"
Name: "{commonstartup}\ReEnvision AI"; Filename: "{app}\ReEnvisionAI.exe"; WorkingDir: "{app}"

[Files]
Source: "ReEnvisionAI.exe"; DestDir: "{app}"; Flags: ignoreversion signonce
Source: "podman-5.4.1-setup.exe"; DestDir: "{tmp}"; Flags: deleteafterinstall
Source: "podman_setup.bat"; DestDir: "{app}"; Flags: ignoreversion
//Source: "podman_setup.ps1"; DestDir: "{app}"; Flags: ignoreversion signonce
Source: "wsl_install.ps1"; DestDir: "{tmp}"; Flags: ignoreversion deleteafterinstall
Source: "post_wsl_install.ps1"; DestDir: "{app}"; Flags: ignoreversion signonce
Source: "config.json"; DestDir: "{localappdata}\ReEnvisionAI\"; Flags: ignoreversion
Source: ".wslconfig"; DestDir: "{%USERPROFILE}"; Flags: ignoreversion

[Run]
Filename: "Powershell.exe"; Parameters: "-ExecutionPolicy Bypass -File '{tmp}\wsl_install.ps1'"; Flags: shellexec waituntilterminated; StatusMsg: "Installing Windows Subsystem for Linux, please wait.."; AfterInstall: SetNeedsReboot

Filename: "{tmp}\podman-5.4.1-setup.exe"; Parameters: "/quiet"; Flags: shellexec  waituntilterminated; StatusMsg: "Installing Podman, please wait..."; BeforeInstall: SetMarqueeProgress(True)

Filename: "{cmd}"; Parameters: "/c cmdkey /generic:ReEnvisionAI/hf_token /user:reai /pass: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"; Flags: runhidden shellexec waituntilterminated

//Filename: "DISM"; Parameters: "/Online /Enable-Feature /FeatureName:Microsoft-Windows-Subsystem-Linux /All /norestart"; Check: NeedsWSLEnable; Flags: waituntilterminated; StatusMsg: "Enabling Windows Subsystem for Linux, please wait..."; AfterInstall: SetNeedsReboot
//Filename: "DISM"; Parameters: "/Online /Enable-Feature /FeatureName:VirtualMachinePlatform /All /norestart"; Check: NeedsVMPlatformEnable; Flags: waituntilterminated; StatusMsg: "Enabling Virtual Machine Platform, please wait..."; AfterInstall: SetNeedsReboot

Filename: "netsh"; Parameters: "advfirewall firewall delete rule name=""ReEnvision AI"""; Flags:  waituntilterminated
Filename: "netsh"; Parameters: "advfirewall firewall add rule name=""ReEnvision AI"" dir=in action=allow protocol=TCP localport={code:GetPort}"; Flags:  waituntilterminated; StatusMsg: "Setting up firewall rule, please wait..."; AfterInstall: SetMarqueeProgress(False)

[Registry]
Root: HKLM; Subkey: "SOFTWARE\ReEnvisionAI\ReEnvisionAI"; ValueType: dword; ValueName: "Port"; ValueData: "{code:GetPort}"; Flags: uninsdeletekey
Root: HKLM; Subkey: "SOFTWARE\Microsoft\Windows\CurrentVersion\RunOnce"; ValueType: string; ValueName: "ReEnvisionAI_Setup"; ValueData: "cmd.exe /c ""{app}\podman_setup.bat"""; Flags: uninsdeletevalue


[UninstallRun]
Filename: "netsh"; Parameters: "advfirewall firewall delete rule name=""ReEnvision AI"""; Flags: waituntilterminated; RunOnceId: "DeleteReEnvisionAIFirewallRule"

[Code]
var
  NeedsReboot: Boolean;
  RandomPort: Integer;
  
procedure SetMarqueeProgress(Marquee: Boolean);
begin
  if Marquee then
  begin
    WizardForm.ProgressGauge.Style := npbstMarquee;
  end
    else
  begin
    WizardForm.ProgressGauge.Style := npbstNormal;
  end;
end;

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


procedure InitializeWizard;
begin
  NeedsReboot := False;
end;

function GetPort(Param: String): String;
begin
  Result := IntToStr(RandomPort);
end;

procedure SetNeedsReboot;
begin
  NeedsReboot := True;
end;

procedure CurStepChanged(CurStep: TSetupStep);
begin
  if CurStep = ssDone then
  begin
    if NeedsReboot then
      MsgBox('A system restart is required to complete the installation. Please restart your computer.', mbInformation, MB_OK);
  end;
end;

function InitializeSetup(): Boolean;
begin
  RandomPort := 31330 + Random(52000 - 31330 + 1);

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

