$ErrorActionPreference = 'Stop'

$Owner = 'malleus35'
$Repo = 'agentcom'
$Version = if ($env:VERSION) { $env:VERSION } else { 'v0.3.0' }
$InstallDir = if ($env:INSTALL_DIR) { $env:INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA 'Programs\agentcom' }

$osArch = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString().ToLowerInvariant()
$arch = switch ($osArch) {
  'x64' { 'amd64' }
  'arm64' { 'arm64' }
  default { throw "Unsupported architecture: $osArch" }
}
$archive = "agentcom_$($Version.TrimStart('v'))_windows_${arch}.zip"
$url = "https://github.com/$Owner/$Repo/releases/download/$Version/$archive"

New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
$tmpDir = Join-Path ([System.IO.Path]::GetTempPath()) ("agentcom-" + [guid]::NewGuid().ToString())
New-Item -ItemType Directory -Force -Path $tmpDir | Out-Null

try {
  $archivePath = Join-Path $tmpDir $archive
  Write-Host "Downloading $url"
  Invoke-WebRequest -Uri $url -OutFile $archivePath
  Expand-Archive -Path $archivePath -DestinationPath $tmpDir -Force
  Copy-Item (Join-Path $tmpDir 'agentcom.exe') (Join-Path $InstallDir 'agentcom.exe') -Force
  Write-Host "Installed agentcom to $InstallDir\agentcom.exe"
  & (Join-Path $InstallDir 'agentcom.exe') version
}
finally {
  Remove-Item -Recurse -Force $tmpDir
}
