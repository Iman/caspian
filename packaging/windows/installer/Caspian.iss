#define AppName "Caspian-BYOC"
#ifndef AppVersion
#define AppVersion "0.2.1"
#endif
#ifndef BuildArchitecture
#define BuildArchitecture "x64"
#endif
#ifndef AllowedArchitecture
#define AllowedArchitecture "x64os"
#endif
#define Publisher "Caspian project"

[Setup]
AppId={{8E6FDF9D-613A-4D7C-A59A-49F144ABAE62}
AppName={#AppName}
AppVersion={#AppVersion}
AppPublisher={#Publisher}
DefaultDirName={autopf}\Caspian
DefaultGroupName=Caspian
ArchitecturesAllowed={#AllowedArchitecture}
ArchitecturesInstallIn64BitMode={#AllowedArchitecture}
PrivilegesRequired=admin
OutputDir=..\..\..\out\installer
OutputBaseFilename=CaspianSetup-{#AppVersion}-windows-{#BuildArchitecture}
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
Source: "payload\{#BuildArchitecture}\caspian.exe"; DestDir: "{app}"; Flags: ignoreversion; BeforeInstall: StopCaspianProcesses
Source: "payload\{#BuildArchitecture}\caspian-tethering.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "payload\{#BuildArchitecture}\CaspianControl.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "payload\{#BuildArchitecture}\wintun.dll"; DestDir: "{app}"; Flags: ignoreversion
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
var
  PasswordPage: TInputQueryWizardPage;

procedure InitializeWizard;
begin
  PasswordPage := CreateInputQueryPage(wpSelectTasks,
    'Protect the Caspian web panel',
    'Choose the password for the web panel.',
    'Use at least 8 characters. You will type this password in your web browser.');
  PasswordPage.Add('Panel password:', True);
  PasswordPage.Add('Type the password again:', True);
end;

function IsFreshInstall: Boolean;
begin
  Result := not FileExists(ExpandConstant('{commonappdata}\Caspian\state.json'));
end;

function ShouldSkipPage(PageID: Integer): Boolean;
begin
  Result := (PageID = PasswordPage.ID) and not IsFreshInstall;
end;

function NextButtonClick(CurPageID: Integer): Boolean;
var
  Password: String;
begin
  Result := True;
  if (CurPageID <> PasswordPage.ID) then
    exit;

  Password := Trim(PasswordPage.Values[0]);
  PasswordPage.Values[0] := Password;
  PasswordPage.Values[1] := Trim(PasswordPage.Values[1]);
  if Length(Password) < 8 then
  begin
    MsgBox('Use at least 8 characters for the panel password.', mbError, MB_OK);
    Result := False;
  end
  else if Password <> PasswordPage.Values[1] then
  begin
    MsgBox('The two passwords do not match. Type them again.', mbError, MB_OK);
    Result := False;
  end;
end;

procedure InstallServices;
var
  ResultCode: Integer;
begin
  WizardForm.StatusLabel.Caption := 'Installing and starting Caspian services...';
  if not Exec(ExpandConstant('{sys}\WindowsPowerShell\v1.0\powershell.exe'),
    '-NoProfile -ExecutionPolicy Bypass -File "' + ExpandConstant('{tmp}\service-install.ps1') +
    '" -InstallDirectory "' + ExpandConstant('{app}') + '" -PasswordFile "' +
    ExpandConstant('{tmp}\panel-password.txt') + '"', '', SW_HIDE,
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
var
  PasswordLines: TArrayOfString;
begin
  if IsFreshInstall then
  begin
    SetArrayLength(PasswordLines, 1);
    PasswordLines[0] := PasswordPage.Values[0];
    if not SaveStringsToUTF8FileWithoutBOM(
      ExpandConstant('{tmp}\panel-password.txt'), PasswordLines, False) then
      RaiseException('Setup could not prepare the panel password.');
  end;
  StopCaspianProcesses;
  Result := '';
end;
