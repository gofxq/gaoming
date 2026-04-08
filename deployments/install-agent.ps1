#Requires -RunAsAdministrator
[CmdletBinding()]
param(
  [ValidateNotNullOrEmpty()][string]$Repo = "gofxq/gaoming",
  [ValidateNotNullOrEmpty()][string]$Version = "latest",
  [ValidateNotNullOrEmpty()][string]$InstallDir = "$env:ProgramFiles\GaomingAgent",
  [ValidateNotNullOrEmpty()][string]$TaskName = "GaomingAgent",
  [ValidateNotNullOrEmpty()][string]$MasterApiUrl = "https://gm-metric.gofxq.com/",
  [ValidateNotNullOrEmpty()][string]$IngestGatewayUrl = "https://gm-metric.gofxq.com/",
  [ValidateNotNullOrEmpty()][string]$IngestGatewayGrpcAddr = "gm-metric.gofxq.com:8091",
  [ValidateSet("http", "grpc")][string]$ReportMode = "http",
  [string]$TenantCode = "",
  [ValidateRange(1, 3600)][int]$LoopIntervalSec = 5,
  [ValidateNotNullOrEmpty()][string]$Region = "local",
  [ValidateNotNullOrEmpty()][string]$EnvName = "prod",
  [ValidateNotNullOrEmpty()][string]$Role = "node"
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$script:WebParams = @{ TimeoutSec = 60 }
if ($PSVersionTable.PSVersion.Major -lt 6) {
  $script:WebParams.UseBasicParsing = $true
}

function Resolve-Arch {
  switch ([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString().ToLowerInvariant()) {
    "x64"   { return "amd64" }
    "arm64" { return "arm64" }
    default { throw "unsupported architecture: $([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture)" }
  }
}

function Get-ReleaseBaseUrl {
  param(
    [string]$Repo,
    [string]$Version
  )

  if ($Version -eq "latest") {
    return "https://github.com/$Repo/releases/latest/download"
  }

  return "https://github.com/$Repo/releases/download/$Version"
}

function Get-InstallTenantCode {
  param([string]$MasterApiUrl)

  $endpoint = $MasterApiUrl.TrimEnd('/') + "/master/api/v1/install/tenant"
  $response = Invoke-RestMethod -Method Post -Uri $endpoint @script:WebParams

  if (-not $response -or [string]::IsNullOrWhiteSpace($response.tenant_code)) {
    throw "tenant allocation response did not include tenant_code"
  }

  return [string]$response.tenant_code
}

function Get-ExistingTenantCode {
  param([string]$ConfigPath)

  if (-not (Test-Path $ConfigPath)) {
    return ""
  }

  $match = Select-String -Path $ConfigPath -Pattern '^\s*tenant_code:\s*"?(?<tenant>[^"\r\n]+)"?\s*$' |
           Select-Object -First 1

  if ($match) {
    return $match.Matches[0].Groups['tenant'].Value
  }

  return ""
}

function Get-ExpectedSha256 {
  param(
    [string]$ChecksumFile,
    [string]$FileName
  )

  $escaped = [regex]::Escape($FileName)
  $match = Select-String -Path $ChecksumFile -Pattern "^(?<hash>[A-Fa-f0-9]{64})\s+\*?$escaped$" |
           Select-Object -First 1

  if (-not $match) {
    throw "checksum entry not found for $FileName"
  }

  return $match.Matches[0].Groups['hash'].Value.ToLowerInvariant()
}

function Assert-SafeInstallDir {
  param([string]$Path)

  $full = [System.IO.Path]::GetFullPath($Path)
  $root = [System.IO.Path]::GetPathRoot($full)

  if ($full -eq $root) {
    throw "unsafe InstallDir: $full"
  }

  return $full
}

$arch = Resolve-Arch
$InstallDir = Assert-SafeInstallDir -Path $InstallDir
$asset = "gaoming-agent_windows_${arch}.zip"
$releaseBaseUrl = Get-ReleaseBaseUrl -Repo $Repo -Version $Version

$tmpDir = Join-Path $env:TEMP ("gaoming-agent-" + [guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $tmpDir -Force | Out-Null

try {
  if ([string]::IsNullOrWhiteSpace($TenantCode)) {
    $TenantCode = Get-ExistingTenantCode -ConfigPath (Join-Path $InstallDir "agent-config.yaml")
  }

  if ([string]::IsNullOrWhiteSpace($TenantCode)) {
    Write-Verbose "allocating tenant from master-api"
    $TenantCode = Get-InstallTenantCode -MasterApiUrl $MasterApiUrl
  }

  $zipPath = Join-Path $tmpDir $asset
  $checksumPath = Join-Path $tmpDir "checksums.txt"

  Invoke-WebRequest -Uri "$releaseBaseUrl/$asset" -OutFile $zipPath @script:WebParams
  Invoke-WebRequest -Uri "$releaseBaseUrl/checksums.txt" -OutFile $checksumPath @script:WebParams

  $expected = Get-ExpectedSha256 -ChecksumFile $checksumPath -FileName $asset
  $actual = (Get-FileHash -Algorithm SHA256 -Path $zipPath).Hash.ToLowerInvariant()

  if ($expected -ne $actual) {
    throw "checksum mismatch for $asset"
  }

  if (Get-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue) {
    Stop-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue
  }

  if (Test-Path $InstallDir) {
    Remove-Item -Path $InstallDir -Recurse -Force
  }

  New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
  Expand-Archive -Path $zipPath -DestinationPath $InstallDir -Force

  $exePath = Join-Path $InstallDir "gaoming-agent.exe"
  if (-not (Test-Path $exePath)) {
    throw "gaoming-agent.exe not found in $InstallDir"
  }

  @"
master_api_url: "$MasterApiUrl"
ingest_gateway_url: "$IngestGatewayUrl"
ingest_gateway_grpc_addr: "$IngestGatewayGrpcAddr"
report_mode: "$ReportMode"
region: "$Region"
env: "$EnvName"
role: "$Role"
tenant_code: "$TenantCode"
loop_interval_sec: $LoopIntervalSec
"@ | Set-Content -Path (Join-Path $InstallDir "agent-config.yaml") -Encoding UTF8

  $action = New-ScheduledTaskAction -Execute $exePath -WorkingDirectory $InstallDir
  $trigger = New-ScheduledTaskTrigger -AtStartup
  $settings = New-ScheduledTaskSettingsSet -StartWhenAvailable -RestartCount 999 -RestartInterval (New-TimeSpan -Minutes 1)
  $principal = New-ScheduledTaskPrincipal -UserId "SYSTEM" -LogonType ServiceAccount -RunLevel Highest

  Register-ScheduledTask `
    -TaskName $TaskName `
    -Action $action `
    -Trigger $trigger `
    -Settings $settings `
    -Principal $principal `
    -Force | Out-Null

  Start-ScheduledTask -TaskName $TaskName

  Write-Output "installed $TaskName to $InstallDir"
  Write-Output "report_mode: $ReportMode"
  if ($ReportMode -eq "grpc") {
    Write-Output "ingest_grpc_addr: $IngestGatewayGrpcAddr"
  }
  Write-Output "tenant_code: $TenantCode"
  Write-Output ("dashboard: " + $MasterApiUrl.TrimEnd('/') + "/" + $TenantCode)
  Write-Output ("hosts api: " + $MasterApiUrl.TrimEnd('/') + "/master/api/v1/hosts?tenant=" + $TenantCode)
}
finally {
  Remove-Item -Path $tmpDir -Recurse -Force -ErrorAction SilentlyContinue
}
