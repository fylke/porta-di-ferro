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
DefaultDirName={autopf}\Porta di Ferro
DefaultGroupName=Porta di Ferro
OutputDir=Output
OutputBaseFilename=porta-di-ferro-{#MyAppVersion}-windows-x64-setup
Compression=lzma2/max
SolidCompression=yes
WizardStyle=modern
; Per-user by default, so no UAC prompt stands between a volunteer and a running
; tournament. An admin install is still possible for a shared machine.
PrivilegesRequiredOverridesAllowed=dialog
PrivilegesRequired=lowest
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
DisableProgramGroupPage=yes
; Nothing here needs deciding by the person installing it, so the wizard gets out of the
; way: the five minutes are for running a tournament, not for reading dialogs.
DisableReadyPage=yes
DisableDirPage=auto

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

[UninstallDelete]
; Tournament data lives in the organizer's own folder and is deliberately left behind:
; it is their record of the event, and it is plain JSON they can read.
Type: dirifempty; Name: "{app}"
