$ErrorActionPreference = 'Stop'

param(
  [string]$Repo = "gofxq/gaoming",
  [string]$Version = "latest",
  [string]$InstallDir = "$env:ProgramFiles\GaomingAgent",
  [string]$TaskName = "GaomingAgent",
  [string]$MasterApiUrl = "https://gm-metric.gofxq.com/",
  [string]$IngestGatewayUrl = "https://gm-metric.gofxq.com/",
  [string]$TenantCode = "",
  [int]$LoopIntervalSec = 5,
  [string]$Region = "local",
  [string]$EnvName = "prod",
  [string]$Role = "node"
)

function Assert-Admin {
  $principal = New-Object Security.Principal.WindowsPrincipal([Security.Principal.WindowsIdentity]::GetCurrent())
  if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    throw "install-agent.ps1 must run as Administrator"
  }
}

function Resolve-Arch {
  switch ([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString().ToLower()) {
    "x64" { "amd64" }
    "arm64" { "arm64" }
    default { throw "unsupported architecture" }
  }
}

Assert-Admin
$arch = Resolve-Arch

$asset = "gaoming-agent_windows_${arch}.zip"
$tmpDir = Join-Path $env:TEMP ("gaoming-agent-" + [guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $tmpDir | Out-Null

try {
  if ($Version -eq "latest") {
    $assetUrl = "https://github.com/$Repo/releases/latest/download/$asset"
    $checksumUrl = "https://github.com/$Repo/releases/latest/download/checksums.txt"
  } else {
    $assetUrl = "https://github.com/$Repo/releases/download/$Version/$asset"
    $checksumUrl = "https://github.com/$Repo/releases/download/$Version/checksums.txt"
  }

  Invoke-WebRequest -Uri $assetUrl -OutFile (Join-Path $tmpDir $asset)
  Invoke-WebRequest -Uri $checksumUrl -OutFile (Join-Path $tmpDir "checksums.txt")

  $expected = (Select-String -Path (Join-Path $tmpDir "checksums.txt") -Pattern $asset).Line.Split(' ')[0]
  $actual = (Get-FileHash -Algorithm SHA256 (Join-Path $tmpDir $asset)).Hash.ToLower()
  if ($expected.ToLower() -ne $actual) {
    throw "checksum mismatch for $asset"
  }

  if (Test-Path $InstallDir) {
    Remove-Item -Recurse -Force $InstallDir
  }
  New-Item -ItemType Directory -Path $InstallDir | Out-Null
  Expand-Archive -Path (Join-Path $tmpDir $asset) -DestinationPath $InstallDir -Force

  @"
master_api_url: "$MasterApiUrl"
ingest_gateway_url: "$IngestGatewayUrl"
region: "$Region"
env: "$EnvName"
role: "$Role"
tenant_code: "$TenantCode"
loop_interval_sec: $LoopIntervalSec
"@ | Set-Content -Path (Join-Path $InstallDir "agent-config.yaml") -Encoding UTF8

  $action = New-ScheduledTaskAction -Execute (Join-Path $InstallDir "gaoming-agent.exe") -WorkingDirectory $InstallDir
  $trigger = New-ScheduledTaskTrigger -AtStartup
  Register-ScheduledTask -TaskName $TaskName -Action $action -Trigger $trigger -User "SYSTEM" -RunLevel Highest -Force | Out-Null
  Start-ScheduledTask -TaskName $TaskName

  Write-Host "installed $TaskName to $InstallDir"
  Write-Host "tenant_code: $TenantCode"
} finally {
  Remove-Item -Recurse -Force $tmpDir -ErrorAction SilentlyContinue
}
