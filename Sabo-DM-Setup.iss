#define MyAppName "Sabo DM"
#define MyAppVersion "1.3.5"
#define MyAppPublisher "Sabo DM"
#define MyAppExeName "Sabo-DM.exe"

[Setup]
AppId={{8E2C42B1-47B0-4F62-9D1A-9A3D2D9B1333}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppPublisher={#MyAppPublisher}
DefaultDirName={autopf}\Sabo DM
DefaultGroupName=Sabo DM
UninstallDisplayIcon={app}\{#MyAppExeName}
OutputDir=dist
OutputBaseFilename=Sabo-DM-Setup
SetupIconFile=sabo.ico
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
PrivilegesRequired=lowest
ArchitecturesInstallIn64BitMode=x64

[Tasks]
Name: "desktopicon"; Description: "Create a desktop shortcut"; GroupDescription: "Additional shortcuts:"
Name: "startup"; Description: "Run Sabo DM at Windows startup (recommended)"; GroupDescription: "Startup:"

[Files]
Source: "Sabo-DM.exe"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{group}\Sabo DM"; Filename: "{app}\{#MyAppExeName}"; IconFilename: "{app}\{#MyAppExeName}"
Name: "{autodesktop}\Sabo DM"; Filename: "{app}\{#MyAppExeName}"; Tasks: desktopicon; IconFilename: "{app}\{#MyAppExeName}"

[Registry]
Root: HKCU; Subkey: "Software\Microsoft\Windows\CurrentVersion\Run"; ValueType: string; ValueName: "Sabo DM"; ValueData: """{app}\{#MyAppExeName}"" --background"; Tasks: startup; Flags: uninsdeletevalue

[Run]
Filename: "{app}\{#MyAppExeName}"; Description: "Launch Sabo DM"; Flags: postinstall nowait skipifsilent
