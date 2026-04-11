#Requires -RunAsAdministrator
[CmdletBinding()]
param(
  [ValidateNotNullOrEmpty()][string]$Repo = "gofxq/gaoming",
  [ValidateNotNullOrEmpty()][string]$Version = "latest",
  [ValidateNotNullOrEmpty()][string]$InstallDir = "$env:ProgramFiles\GaomingAgent",
  [ValidateNotNullOrEmpty()][string]$TaskName = "GaomingAgent",
  [Alias("MasterApiUrl")][ValidateNotNullOrEmpty()][string]$WebBaseUrl = "https://gm-metric.gofxq.com/",
  [ValidateNotNullOrEmpty()][string]$IngestGatewayGrpcAddr = "gm-rpc.gofxq.com:443",
  [string]$TenantCode = "",
  [ValidateRange(1, 3600)][int]$LoopIntervalSec = 5,
  [ValidateNotNullOrEmpty()][string]$Region = "local",
  [ValidateNotNullOrEmpty()][string]$EnvName = "prod",
  [ValidateNotNullOrEmpty()][string]$Role = "node",
  [switch]$NoPrompt
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$script:WebParams = @{ TimeoutSec = 60 }
if ($PSVersionTable.PSVersion.Major -lt 6) {
  $script:WebParams.UseBasicParsing = $true
}

function Write-Info {
  param([string]$Message)
  Write-Host "[+] $Message"
}

function Test-CanPrompt {
  try {
    return $Host.Name -ne "ServerRemoteHost"
  }
  catch {
    return $false
  }
}

function Read-ValueWithDefault {
  param(
    [string]$Label,
    [string]$CurrentValue,
    [string]$DisplayDefault
  )

  if ($NoPrompt -or -not (Test-CanPrompt)) {
    return $CurrentValue
  }

  $inputValue = Read-Host "$Label [$DisplayDefault]"
  if ([string]::IsNullOrWhiteSpace($inputValue)) {
    return $CurrentValue
  }
  return $inputValue.Trim()
}

function Assert-Repo {
  param([string]$Value)
  if ($Value -notmatch '^[^/]+/[^/]+$') {
    throw "repo must be in owner/name format: $Value"
  }
}

function Assert-HttpUrl {
  param(
    [string]$Value,
    [string]$Name
  )

  $uri = $null
  if (-not [Uri]::TryCreate($Value, [UriKind]::Absolute, [ref]$uri) -or ($uri.Scheme -ne "http" -and $uri.Scheme -ne "https")) {
    throw "$Name must start with http:// or https://: $Value"
  }
}

function Assert-NonEmpty {
  param(
    [string]$Value,
    [string]$Name
  )

  if ([string]::IsNullOrWhiteSpace($Value)) {
    throw "$Name must not be empty"
  }
}

function Assert-PositiveInt {
  param(
    [int]$Value,
    [string]$Name
  )

  if ($Value -le 0) {
    throw "$Name must be greater than 0"
  }
}

function Assert-SafeInstallDir {
  param([string]$Path)

  $full = [System.IO.Path]::GetFullPath($Path)
  $root = [System.IO.Path]::GetPathRoot($full)
  if ([string]::Equals($full.TrimEnd('\'), $root.TrimEnd('\'), [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "unsafe InstallDir: $full"
  }

  return $full
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

function Get-ExistingTenantCode {
  param([string]$ConfigPath)

  if (-not (Test-Path -LiteralPath $ConfigPath)) {
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

  foreach ($line in Get-Content -LiteralPath $ChecksumFile) {
    if ([string]::IsNullOrWhiteSpace($line)) {
      continue
    }

    $parts = $line -split '\s+', 2
    if ($parts.Count -lt 2) {
      continue
    }

    $candidate = $parts[1].TrimStart('*')
    $candidate = [System.IO.Path]::GetFileName($candidate)
    if ($candidate -eq $FileName) {
      return $parts[0].ToLowerInvariant()
    }
  }

  throw "checksum entry not found for $FileName"
}

function Render-AgentConfig {
  param(
    [string]$MasterApiUrl,
    [string]$IngestGatewayGrpcAddr,
    [string]$Region,
    [string]$EnvName,
    [string]$Role,
    [string]$TenantCode,
    [int]$LoopIntervalSec
  )

  return @"
master_api_url: "$MasterApiUrl"
ingest_gateway_grpc_addr: "$IngestGatewayGrpcAddr"
region: "$Region"
env: "$EnvName"
role: "$Role"
tenant_code: "$TenantCode"
loop_interval_sec: $LoopIntervalSec
"@
}

function Set-Utf8NoBomContent {
  param(
    [string]$Path,
    [string]$Content
  )

  $encoding = New-Object System.Text.UTF8Encoding($false)
  [System.IO.File]::WriteAllText($Path, $Content, $encoding)
}

function Build-DashboardUrl {
  param(
    [string]$WebBaseUrl,
    [string]$TenantCode
  )

  return $WebBaseUrl.TrimEnd('/') + "/" + $TenantCode
}

function Build-HostsApiUrl {
  param(
    [string]$WebBaseUrl,
    [string]$TenantCode
  )

  return $WebBaseUrl.TrimEnd('/') + "/master/api/v1/hosts?tenant=" + $TenantCode
}

function Wait-ForTenantCode {
  param([string]$ConfigPath)

  for ($attempt = 0; $attempt -lt 10; $attempt++) {
    $tenantCode = Get-ExistingTenantCode -ConfigPath $ConfigPath
    if (-not [string]::IsNullOrWhiteSpace($tenantCode)) {
      return $tenantCode
    }
    Start-Sleep -Seconds 1
  }

  return ""
}

$webBaseExplicit = $PSBoundParameters.ContainsKey("WebBaseUrl") -or $PSBoundParameters.ContainsKey("MasterApiUrl")
$ingestGrpcExplicit = $PSBoundParameters.ContainsKey("IngestGatewayGrpcAddr")
$tenantExplicit = $PSBoundParameters.ContainsKey("TenantCode")
$loopIntervalExplicit = $PSBoundParameters.ContainsKey("LoopIntervalSec")

$InstallDir = Assert-SafeInstallDir -Path $InstallDir
$configPath = Join-Path $InstallDir "agent-config.yaml"

if (-not $tenantExplicit -and [string]::IsNullOrWhiteSpace($TenantCode)) {
  $TenantCode = Get-ExistingTenantCode -ConfigPath $configPath
}

if (-not $webBaseExplicit) {
  $WebBaseUrl = Read-ValueWithDefault -Label "web-url" -CurrentValue $WebBaseUrl -DisplayDefault $WebBaseUrl
}
if (-not $ingestGrpcExplicit) {
  $IngestGatewayGrpcAddr = Read-ValueWithDefault -Label "ingest-grpc-addr" -CurrentValue $IngestGatewayGrpcAddr -DisplayDefault $IngestGatewayGrpcAddr
}
if (-not $tenantExplicit) {
  $tenantDisplay = $TenantCode
  if ([string]::IsNullOrWhiteSpace($tenantDisplay)) {
    $tenantDisplay = "<auto>"
  }
  $TenantCode = Read-ValueWithDefault -Label "tenant" -CurrentValue $TenantCode -DisplayDefault $tenantDisplay
}
if (-not $loopIntervalExplicit) {
  $loopInput = Read-ValueWithDefault -Label "loop-interval-sec" -CurrentValue ([string]$LoopIntervalSec) -DisplayDefault ([string]$LoopIntervalSec)
  $parsedLoopInterval = 0
  if (-not [int]::TryParse($loopInput, [ref]$parsedLoopInterval)) {
    throw "loop-interval-sec must be a positive integer"
  }
  $LoopIntervalSec = $parsedLoopInterval
}

Assert-Repo -Value $Repo
Assert-HttpUrl -Value $WebBaseUrl -Name "web-url"
Assert-NonEmpty -Value $IngestGatewayGrpcAddr -Name "ingest-grpc-addr"
Assert-PositiveInt -Value $LoopIntervalSec -Name "loop-interval-sec"

$arch = Resolve-Arch
$asset = "gaoming-agent_windows_${arch}.zip"
$releaseBaseUrl = Get-ReleaseBaseUrl -Repo $Repo -Version $Version
$tmpDir = Join-Path ([System.IO.Path]::GetTempPath()) ("gaoming-agent-" + [guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $tmpDir -Force | Out-Null

try {
  $zipPath = Join-Path $tmpDir $asset
  $checksumPath = Join-Path $tmpDir "checksums.txt"

  Write-Info "downloading $asset"
  Invoke-WebRequest -Uri "$releaseBaseUrl/$asset" -OutFile $zipPath @script:WebParams
  Invoke-WebRequest -Uri "$releaseBaseUrl/checksums.txt" -OutFile $checksumPath @script:WebParams

  $expected = Get-ExpectedSha256 -ChecksumFile $checksumPath -FileName $asset
  $actual = (Get-FileHash -Algorithm SHA256 -Path $zipPath).Hash.ToLowerInvariant()
  if ($expected -ne $actual) {
    throw "checksum mismatch for $asset"
  }

  if (Get-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue) {
    Stop-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue
    Unregister-ScheduledTask -TaskName $TaskName -Confirm:$false
  }

  if (Test-Path -LiteralPath $InstallDir) {
    Remove-Item -LiteralPath $InstallDir -Recurse -Force
  }

  New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
  Expand-Archive -Path $zipPath -DestinationPath $InstallDir -Force

  $exePath = Join-Path $InstallDir "gaoming-agent.exe"
  if (-not (Test-Path -LiteralPath $exePath)) {
    throw "gaoming-agent.exe not found in $InstallDir"
  }

  $configBody = Render-AgentConfig `
    -MasterApiUrl $WebBaseUrl `
    -IngestGatewayGrpcAddr $IngestGatewayGrpcAddr `
    -Region $Region `
    -EnvName $EnvName `
    -Role $Role `
    -TenantCode $TenantCode `
    -LoopIntervalSec $LoopIntervalSec
  Set-Utf8NoBomContent -Path $configPath -Content $configBody

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

  $effectiveTenantCode = $TenantCode
  if ([string]::IsNullOrWhiteSpace($effectiveTenantCode)) {
    $effectiveTenantCode = Wait-ForTenantCode -ConfigPath $configPath
  }

  Write-Output "installed $TaskName to $InstallDir"
  Write-Output "config: $configPath"
  Write-Output "ingest_grpc_addr: $IngestGatewayGrpcAddr"
  if (-not [string]::IsNullOrWhiteSpace($effectiveTenantCode)) {
    Write-Output "tenant_code: $effectiveTenantCode"
    Write-Output ("dashboard: " + (Build-DashboardUrl -WebBaseUrl $WebBaseUrl -TenantCode $effectiveTenantCode))
    Write-Output ("hosts api: " + (Build-HostsApiUrl -WebBaseUrl $WebBaseUrl -TenantCode $effectiveTenantCode))
  }
  else {
    Write-Output "tenant_code: <pending>"
    Write-Output "tenant_code will be requested from master-api at runtime; if that fails, agent will generate one locally"
  }
}
finally {
  Remove-Item -Path $tmpDir -Recurse -Force -ErrorAction SilentlyContinue
}
