# Sabo DM — Step 1 Windows Verification

This checklist is for the actual Windows/WebView2 application. Passing the FFmpeg smoke test alone does not close Step 1.

## Automated checks

1. Install Go 1.23 or newer.
2. Ensure `ffmpeg.exe` and `ffprobe.exe` are installed together in `%APPDATA%\Sabo DM\tools`, beside the application, or on `PATH`.
3. Open PowerShell in this source folder and run:

```powershell
go test ./...
go build -trimpath -o Sabo-DM-Step1-Verification.exe .
```

4. Run the codec smoke test below. It creates a short test tone in a temporary folder, converts it to each advertised output family, and verifies each result with FFprobe.

```powershell
$ErrorActionPreference = 'Stop'
$ffmpeg = (Get-Command ffmpeg.exe -ErrorAction Stop).Source
$ffprobe = (Get-Command ffprobe.exe -ErrorAction Stop).Source
$testDir = Join-Path $env:TEMP 'SaboDM-Audio-Step1-Test'
New-Item -ItemType Directory -Force -Path $testDir | Out-Null
$source = Join-Path $testDir 'source.wav'
& $ffmpeg -hide_banner -loglevel error -y -f lavfi -i 'sine=frequency=880:duration=4' -c:a pcm_s16le $source
if ($LASTEXITCODE -ne 0) { throw 'Could not create test audio.' }
$cases = @(
  @{Name='MP3'; Ext='mp3'; Codec='libmp3lame'; Format='mp3'; Rates=@('128k','192k','320k')},
  @{Name='M4A'; Ext='m4a'; Codec='aac'; Format='ipod'; Rates=@('128k','192k','256k','320k')},
  @{Name='AAC'; Ext='aac'; Codec='aac'; Format='adts'; Rates=@('128k','192k','256k','320k')},
  @{Name='Opus'; Ext='opus'; Codec='libopus'; Format='opus'; Rates=@('128k','160k','192k','256k')},
  @{Name='OGG'; Ext='ogg'; Codec='libvorbis'; Format='ogg'; Rates=@('128k','192k')},
  @{Name='WMA'; Ext='wma'; Codec='wmav2'; Format='asf'; Rates=@('128k','192k','320k')}
)
foreach ($case in $cases) {
  foreach ($rate in $case.Rates) {
    $out = Join-Path $testDir ($case.Name + '-' + $rate + '.' + $case.Ext)
    & $ffmpeg -hide_banner -loglevel error -y -i $source -map '0:a:0' -vn -c:a $case.Codec -b:a $rate -f $case.Format $out
    if ($LASTEXITCODE -ne 0 -or !(Test-Path $out) -or (Get-Item $out).Length -eq 0) { throw "FFmpeg failed: $($case.Name) $rate" }
    & $ffprobe -v error -select_streams 'a:0' -show_entries stream=codec_name -of csv=p=0 $out | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "FFprobe failed: $out" }
  }
}
$lossless = @(
  @{Ext='wav'; Codec='pcm_s16le'},
  @{Ext='flac'; Codec='flac'}
)
foreach ($case in $lossless) {
  $out = Join-Path $testDir ('lossless.' + $case.Ext)
  & $ffmpeg -hide_banner -loglevel error -y -i $source -map '0:a:0' -vn -c:a $case.Codec $out
  if ($LASTEXITCODE -ne 0 -or !(Test-Path $out) -or (Get-Item $out).Length -eq 0) { throw "FFmpeg failed: $($case.Ext)" }
  & $ffprobe -v error -select_streams 'a:0' -show_entries stream=codec_name -of csv=p=0 $out | Out-Null
  if ($LASTEXITCODE -ne 0) { throw "FFprobe failed: $out" }
}
Write-Host "PASS: all audio codec smoke tests. Results: $testDir" -ForegroundColor Green
```

## Manual WebView2 end-to-end checks

- Download a video that has an audio stream. The dashboard should show the extraction prompt after the download reaches `Done`.
- Choose MP3 VBR and verify the separate `_audio.mp3` file plays.
- Repeat for M4A, AAC, Opus, OGG, WAV, FLAC and WMA.
- Repeat with each advertised bitrate preset. OGG is intentionally limited to 128/192 kbps in this build; Opus is limited to 256 kbps maximum.
- Confirm the original video still plays and its size/time have not changed.
- Download a direct audio file and convert it to a different format.
- Confirm progress advances, reaches 100%, and the output can be opened.
- Repeat when an output with the same name already exists; Sabo DM must create a numbered filename and preserve the existing file.
- Try a file with no audio stream; the UI must show a clear error and must not create an empty output.
- Try with FFmpeg/FFprobe unavailable; conversion must be blocked with a clear install instruction.
- Run a normal video download and normal direct-file download afterward to check for regressions.

**Step 1 is complete only after Go tests, Windows build, codec smoke tests, and the manual WebView2 checks all pass.**
