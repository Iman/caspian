#define AppName "Caspian-BYOC"
#ifndef AppVersion
#define AppVersion "0.2.1"
#endif
#define Publisher "Caspian project"

[Setup]
AppId={{8E6FDF9D-613A-4D7C-A59A-49F144ABAE62}
AppName={#AppName}
AppVersion={#AppVersion}
AppPublisher={#Publisher}
DefaultDirName={autopf}\Caspian
DefaultGroupName=Caspian
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
PrivilegesRequired=admin
OutputDir=..\..\..\out\installer
OutputBaseFilename=CaspianSetup-{#AppVersion}-windows-x64
Compression=lzma2/max
SolidCompression=yes
WizardStyle=modern
LicenseFile=..\..\..\LICENSE
InfoBeforeFile=INSTALL-NOTES.txt
UninstallDisplayIcon={app}\CaspianControl.exe
SetupLogging=yes
SetupIconFile=..\caspian.ico
CloseApplications=no
RestartApplications=no

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "desktopicon"; Description: "Create a desktop shortcut"; GroupDescription: "Shortcuts:"; Flags: unchecked
Name: "startupicon"; Description: "Start Caspian Control when I sign in"; GroupDescription: "Shortcuts:"; Flags: unchecked

[Dirs]
Name: "{commonappdata}\Caspian"

[Files]
Source: "payload\caspian.exe"; DestDir: "{app}"; Flags: ignoreversion; BeforeInstall: StopCaspianProcesses
Source: "payload\caspian-tethering.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "payload\CaspianControl.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "payload\wintun.dll"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\..\..\NOTICE"; DestDir: "{app}"; DestName: "NOTICE.txt"; Flags: ignoreversion
Source: "..\..\..\LICENSE"; DestDir: "{app}"; DestName: "LICENSE.txt"; Flags: ignoreversion
Source: "..\..\..\third_party\wintun\PREBUILT-BINARIES-LICENSE.txt"; DestDir: "{app}"; DestName: "WINTUN-LICENSE.txt"; Flags: ignoreversion
Source: "..\..\..\third_party\dotnet\LICENSE.txt"; DestDir: "{app}"; DestName: "DOTNET-LICENSE.txt"; Flags: ignoreversion
Source: "..\..\..\third_party\dotnet\ThirdPartyNotices.txt"; DestDir: "{app}"; DestName: "DOTNET-THIRD-PARTY-NOTICES.txt"; Flags: ignoreversion
Source: "service-install.ps1"; DestDir: "{tmp}"; Flags: deleteafterinstall; AfterInstall: InstallServices

[InstallDelete]
Type: files; Name: "{app}\Caspian Control.exe"
Type: files; Name: "{commondesktop}\Caspian Control.lnk"

[Icons]
Name: "{group}\Caspian Control"; Filename: "{app}\CaspianControl.exe"; WorkingDir: "{app}"
Name: "{autodesktop}\Caspian Control"; Filename: "{app}\CaspianControl.exe"; WorkingDir: "{app}"; Tasks: desktopicon
Name: "{commonstartup}\Caspian Control"; Filename: "{app}\CaspianControl.exe"; WorkingDir: "{app}"; Tasks: startupicon

[Run]
Filename: "{app}\CaspianControl.exe"; Description: "Open Caspian Control"; Verb: runas; Flags: shellexec postinstall nowait skipifsilent

[UninstallRun]
Filename: "{sys}\sc.exe"; Parameters: "stop caspian-panel"; Flags: runhidden waituntilterminated; RunOnceId: "StopPanel"
Filename: "{sys}\sc.exe"; Parameters: "stop caspian"; Flags: runhidden waituntilterminated; RunOnceId: "StopCore"
Filename: "{sys}\sc.exe"; Parameters: "delete caspian-panel"; Flags: runhidden waituntilterminated; RunOnceId: "DeletePanel"
Filename: "{sys}\sc.exe"; Parameters: "delete caspian"; Flags: runhidden waituntilterminated; RunOnceId: "DeleteCore"

[Code]
procedure InstallServices;
var
  ResultCode: Integer;
begin
  WizardForm.StatusLabel.Caption := 'Installing and starting Caspian services...';
  if not Exec(ExpandConstant('{sys}\WindowsPowerShell\v1.0\powershell.exe'),
    '-NoProfile -ExecutionPolicy Bypass -File "' + ExpandConstant('{tmp}\service-install.ps1') +
    '" -InstallDirectory "' + ExpandConstant('{app}') + '"', '', SW_HIDE,
    ewWaitUntilTerminated, ResultCode) or (ResultCode <> 0) then
    RaiseException('Caspian service setup failed with exit code ' + IntToStr(ResultCode) + '.');
end;

procedure StopCaspianProcesses;
var
  ResultCode: Integer;
begin
  Exec(ExpandConstant('{sys}\taskkill.exe'), '/F /IM CaspianControl.exe', '', SW_HIDE, ewWaitUntilTerminated, ResultCode);
  Exec(ExpandConstant('{sys}\sc.exe'), 'config caspian-panel start= disabled', '', SW_HIDE, ewWaitUntilTerminated, ResultCode);
  Exec(ExpandConstant('{sys}\sc.exe'), 'config caspian start= disabled', '', SW_HIDE, ewWaitUntilTerminated, ResultCode);
  Exec(ExpandConstant('{sys}\sc.exe'), 'stop caspian-panel', '', SW_HIDE, ewWaitUntilTerminated, ResultCode);
  Exec(ExpandConstant('{sys}\sc.exe'), 'stop caspian', '', SW_HIDE, ewWaitUntilTerminated, ResultCode);
  Sleep(5000);
  Exec(ExpandConstant('{sys}\taskkill.exe'), '/F /IM caspian.exe', '', SW_HIDE, ewWaitUntilTerminated, ResultCode);
  Sleep(1000);
end;

function PrepareToInstall(var NeedsRestart: Boolean): String;
begin
  StopCaspianProcesses;
  Result := '';
end;
