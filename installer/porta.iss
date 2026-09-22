; Porta di Ferro -- Windows installer.
;
; The whole premise of the project is that a club volunteer can go from a download to a
; running tournament in under five minutes, on a machine with no tooling on it. This file
; is the last part of that path, which is why it is a deliverable rather than end-stage
; packaging (docs/design.md §1).
;
; Built by .github/workflows/release.yml. Locally:
;   cd web && npm ci && npm run build
;   go build -o dist/porta.exe ./cmd/porta
;   iscc /DMyAppVersion="v0.0.0-local" installer\porta.iss

#ifndef MyAppVersion
  #define MyAppVersion "v0.0.0-dev"
#endif

#define MyAppName "Porta di Ferro"
#define MyAppExeName "porta.exe"

[Setup]
AppId={{A0F1E6C2-6B4C-4C1E-9C1B-2C7A5D8E4F31}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppPublisher=MSL - Medeltida Stridsteknik Linkoping IF
AppPublisherURL=https://github.com/fylke/porta-di-ferro
DefaultDirName={localappdata}\Programs\Porta di Ferro
DefaultGroupName=Porta di Ferro
OutputDir=Output
OutputBaseFilename=porta-di-ferro-{#MyAppVersion}-windows-x64-setup
Compression=lzma2/max
SolidCompression=yes
WizardStyle=modern
; Per-user, always. No UAC prompt and no elevation stands between a volunteer and a
; running tournament, and nothing here needs to be installed machine-wide: the server runs
; as the organizer, reads its own data directory, and binds a high port.
;
; PrivilegesRequiredOverridesAllowed=dialog was tried and removed. It adds Inno's "install
; for all users / just me" dialog, and that is one more decision a volunteer should not have
; to make: there is no good reason to pick either answer here, and the five minutes are for
; running a tournament rather than reading dialogs.
PrivilegesRequired=lowest
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
DisableProgramGroupPage=yes
; Nothing here needs deciding by the person installing it, so the wizard gets out of the
; way: the five minutes are for running a tournament, not for reading dialogs.
DisableReadyPage=yes
DisableDirPage=yes
; Upgrading over a running copy is the normal case: an organizer installs the new version
; on the morning of the event, with yesterday's still in the tray.
;
; The default asks whether to close it and then cannot. Restart Manager closes an
; application by asking its windows to close, and this one has none to ask: the server is
; a console process whose only interface is a notification-area icon on a message-only
; window. So Setup reported success, the file stayed locked, and the install failed until
; the organizer found Task Manager themselves (issue #84). force skips the question and
; ends the process, which is what the honest answer to that question would have been.
CloseApplications=force
CloseApplicationsFilter=*.exe
; And Setup does not put it back afterwards: the [Run] entry below starts the new version,
; and Restart Manager reviving the old one as well would leave two servers fighting over
; the same port.
RestartApplications=no

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "desktopicon"; Description: "Create a shortcut on the desktop"; GroupDescription: "Shortcuts:"

[Files]
Source: "..\dist\{#MyAppExeName}"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\README.md"; DestDir: "{app}"; DestName: "README.txt"; Flags: ignoreversion
Source: "..\LICENSE"; DestDir: "{app}"; DestName: "LICENSE.txt"; Flags: ignoreversion

[Icons]
Name: "{group}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"
Name: "{autodesktop}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; Tasks: desktopicon

[Run]
; Starting it straight away is the point: the server opens the organizer's browser itself,
; and that page shows the LAN address and a QR code for the score keepers' devices.
Filename: "{app}\{#MyAppExeName}"; Description: "Start Porta di Ferro"; Flags: nowait postinstall skipifsilent

[Code]
// Belt and braces for the same problem. CloseApplications=force covers the copy Restart
// Manager can see -- the one holding {app}\porta.exe open. It does not cover a second
// discipline started from the organizer page: that is another porta.exe from the same
// file, and whether Restart Manager enumerates it depends on what it has open at the
// moment Setup looks.
//
// An install that half-succeeds because one of two servers survived is worse than one
// that says what is wrong, so this ends every copy the organizer is running and waits to
// see it gone before any file is replaced. Per-user, so only this account's processes are
// in reach, which is the same account the shortcut starts them under.

const
  ExeName = '{#MyAppExeName}';
  KillTimeoutMS = 10000;

function RunHidden(Exe, Params: String; var Code: Integer): Boolean;
begin
  Result := Exec(Exe, Params, '', SW_HIDE, ewWaitUntilTerminated, Code);
end;

// find sets ERRORLEVEL 1 when it matches nothing, which is the whole test.
function StillRunning(): Boolean;
var
  Code: Integer;
begin
  Result := False;
  if RunHidden(ExpandConstant('{cmd}'),
       '/C tasklist /FI "IMAGENAME eq ' + ExeName + '" /NH | find /I "' + ExeName + '" > nul',
       Code) then
    Result := (Code = 0);
end;

function PrepareToInstall(var NeedsRestart: Boolean): String;
var
  Code, Waited: Integer;
begin
  Result := '';
  NeedsRestart := False;
  if not StillRunning() then
    Exit;

  // /T takes any process it started with it, which is how the sibling disciplines go.
  RunHidden(ExpandConstant('{sys}\taskkill.exe'), '/F /T /IM ' + ExeName, Code);

  Waited := 0;
  while (Waited < KillTimeoutMS) and StillRunning() do
  begin
    Sleep(250);
    Waited := Waited + 250;
  end;

  if StillRunning() then
    Result := 'Porta di Ferro is still running and Setup could not close it.'#13#10#13#10 +
              'Quit it from its icon in the notification area, or end porta.exe in Task '#13#10 +
              'Manager, and then run this installer again.';
end;

[UninstallDelete]
; Tournament data lives in the organizer's own folder and is deliberately left behind:
; it is their record of the event, and it is plain JSON they can read.
Type: dirifempty; Name: "{app}"
