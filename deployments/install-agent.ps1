[CmdletBinding()]
param(
  [ValidateNotNullOrEmpty()][string]$Repo = "gofxq/gaoming",
  [ValidateNotNullOrEmpty()][string]$Version = "latest",
  [ValidateNotNullOrEmpty()][string]$InstallDir = "$env:LOCALAPPDATA\GaomingAgent",
  [ValidateNotNullOrEmpty()][string]$TaskName = "GaomingAgent",
  [Alias("MasterApiUrl")][ValidateNotNullOrEmpty()][string]$WebBaseUrl = "https://gm-metric.gofxq.com/",
  [ValidateNotNullOrEmpty()][string]$IngestGatewayGrpcAddr = "gm-rpc.gofxq.com:443",
  [string]$TenantCode = "",
  [ValidateRange(1, 3600)][int]$LoopIntervalSec = 5,
  [ValidateNotNullOrEmpty()][string]$Region = "local",
  [ValidateNotNullOrEmpty()][string]$EnvName = "prod",
  [ValidateNotNullOrEmpty()][string]$Role = "node",
  [switch]$UpdateBinary,
  [switch]$Reinstall,
  [switch]$NoPrompt
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$script:WebParams = @{ TimeoutSec = 60 }
$script:WinSWVersion = "v2.12.0"
$script:BootstrapLogPath = Join-Path ([System.IO.Path]::GetTempPath()) "gaoming-install-agent.log"
if ($PSVersionTable.PSVersion.Major -lt 6) {
  $script:WebParams.UseBasicParsing = $true
}

function Write-BootstrapLog {
  param([string]$Message)

  $line = "{0} {1}" -f (Get-Date -Format "yyyy-MM-dd HH:mm:ss"), $Message
  try {
    Add-Content -LiteralPath $script:BootstrapLogPath -Value $line -Encoding UTF8
  }
  catch {
  }
  Write-Host $line
}

function Test-IsAdministrator {
  $principal = New-Object Security.Principal.WindowsPrincipal([Security.Principal.WindowsIdentity]::GetCurrent())
  return $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

function Get-CurrentPowerShellExe {
  $processPath = (Get-Process -Id $PID).Path
  if (-not [string]::IsNullOrWhiteSpace($processPath)) {
    return $processPath
  }

  if ($PSVersionTable.PSVersion.Major -ge 6) {
    return "pwsh.exe"
  }
  return "powershell.exe"
}

function ConvertTo-PowerShellLiteral {
  param([object]$Value)

  if ($null -eq $Value) {
    return "''"
  }

  return "'" + ($Value.ToString().Replace("'", "''")) + "'"
}

function Get-ScriptRelaunchPath {
  if (-not [string]::IsNullOrWhiteSpace($PSCommandPath)) {
    Write-BootstrapLog ("using script path for elevation: " + $PSCommandPath)
    return $PSCommandPath
  }

  $definition = $MyInvocation.MyCommand.Definition
  if ([string]::IsNullOrWhiteSpace($definition)) {
    throw "cannot determine script content for elevation"
  }

  $tempPath = Join-Path ([System.IO.Path]::GetTempPath()) ("gaoming-install-agent-elevated-" + [guid]::NewGuid().ToString("N") + ".ps1")
  $encoding = New-Object System.Text.UTF8Encoding($false)
  [System.IO.File]::WriteAllText($tempPath, $definition, $encoding)
  Write-BootstrapLog ("materialized elevated script to: " + $tempPath)
  return $tempPath
}

function Ensure-Elevated {
  param(
    [hashtable]$BoundParameters,
    [string[]]$SwitchParameters
  )

  if (Test-IsAdministrator) {
    Write-BootstrapLog "running with administrator privileges"
    return
  }

  $relaunchPath = Get-ScriptRelaunchPath
  $args = @(
    "-NoProfile",
    "-ExecutionPolicy", "Bypass",
    "-File", $relaunchPath
  )

  foreach ($entry in $BoundParameters.GetEnumerator()) {
    if ($SwitchParameters -contains $entry.Key) {
      if ([bool]$entry.Value) {
        $args += "-" + $entry.Key
      }
      continue
    }

    $args += "-" + $entry.Key
    $args += [string]$entry.Value
  }

  $argumentLine = ($args | ForEach-Object { ConvertTo-PowerShellLiteral -Value $_ }) -join " "
  Write-BootstrapLog ("requesting UAC elevation; log file: " + $script:BootstrapLogPath)
  Start-Process -FilePath (Get-CurrentPowerShellExe) -Verb RunAs -ArgumentList $argumentLine | Out-Null
  exit 0
}

Ensure-Elevated -BoundParameters $PSBoundParameters -SwitchParameters @("UpdateBinary", "Reinstall", "NoPrompt")

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
  $architecture = ""

  try {
    $runtimeInfoType = [System.Type]::GetType("System.Runtime.InteropServices.RuntimeInformation")
    if ($null -ne $runtimeInfoType) {
      $archProperty = $runtimeInfoType.GetProperty("OSArchitecture")
      if ($null -ne $archProperty) {
        $architecture = $archProperty.GetValue($null, $null).ToString()
      }
    }
  }
  catch {
  }

  if ([string]::IsNullOrWhiteSpace($architecture)) {
    $envArch = $env:PROCESSOR_ARCHITEW6432
    if ([string]::IsNullOrWhiteSpace($envArch)) {
      $envArch = $env:PROCESSOR_ARCHITECTURE
    }
    $architecture = $envArch
  }

  switch ($architecture.ToLowerInvariant()) {
    "amd64" { return "amd64" }
    "x64"   { return "amd64" }
    "arm64" { return "arm64" }
    default { throw "unsupported architecture: $architecture" }
  }
}

function Get-WinSWAssetName {
  param([string]$Arch)

  switch ($Arch) {
    "amd64" { return "WinSW-x64.exe" }
    "arm64" { return "WinSW.NET461.exe" }
    default { throw "unsupported WinSW architecture: $Arch" }
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

function Get-ConfigValue {
  param(
    [string]$ConfigPath,
    [string]$Key
  )

  if (-not (Test-Path -LiteralPath $ConfigPath)) {
    return ""
  }

  $escapedKey = [regex]::Escape($Key)
  $pattern = '^\s*' + $escapedKey + ':\s*"?(?<value>[^"\r\n]+)"?\s*$'
  $match = Select-String -Path $ConfigPath -Pattern $pattern | Select-Object -First 1
  if ($match) {
    return $match.Matches[0].Groups['value'].Value.Trim()
  }

  return ""
}

function Get-InstallMode {
  param(
    [string]$ConfigPath,
    [bool]$CanPrompt,
    [bool]$UpdateBinary,
    [bool]$Reinstall
  )

  if ($UpdateBinary -and $Reinstall) {
    throw "UpdateBinary and Reinstall cannot be used together"
  }

  $configExists = Test-Path -LiteralPath $ConfigPath
  if ($UpdateBinary) {
    if (-not $configExists) {
      throw "cannot update binary only; installed config not found: $ConfigPath"
    }
    return "update-binary"
  }

  if ($Reinstall) {
    return "reinstall"
  }

  if (-not $configExists) {
    return "reinstall"
  }

  if (-not $CanPrompt) {
    return "update-binary"
  }

  while ($true) {
    Write-Host "existing config found: $ConfigPath"
    $choice = Read-Host "choose install mode: [1] update binary only (default), [2] reinstall"
    if ($null -eq $choice) {
      $choice = ""
    }
    switch ($choice.Trim().ToLowerInvariant()) {
      "" { return "update-binary" }
      "1" { return "update-binary" }
      "u" { return "update-binary" }
      "update" { return "update-binary" }
      "update-binary" { return "update-binary" }
      "2" { return "reinstall" }
      "r" { return "reinstall" }
      "reinstall" { return "reinstall" }
    }
  }
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

function Get-ServiceWrapperExePath {
  param([string]$InstallDir)

  return Join-Path $InstallDir "gaoming-agent-service.exe"
}

function Get-ServiceWrapperConfigPath {
  param([string]$InstallDir)

  return Join-Path $InstallDir "gaoming-agent-service.xml"
}

function Convert-ToXmlEscapedText {
  param([string]$Value)

  if ($null -eq $Value) {
    return ""
  }

  return [System.Security.SecurityElement]::Escape($Value)
}

function Render-ServiceWrapperConfig {
  param([string]$TaskName)

  $escapedTaskName = Convert-ToXmlEscapedText -Value $TaskName

  return @"
<service>
  <id>$escapedTaskName</id>
  <name>$escapedTaskName</name>
  <description>Gaoming Agent Windows Service</description>
  <executable>%BASE%\gaoming-agent.exe</executable>
  <workingdirectory>%BASE%</workingdirectory>
  <startmode>Automatic</startmode>
  <logpath>%BASE%\logs</logpath>
  <log mode="roll-by-size">
    <sizeThreshold>10485760</sizeThreshold>
    <keepFiles>5</keepFiles>
  </log>
</service>
"@
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

function Test-ServiceExists {
  param([string]$TaskName)

  return $null -ne (Get-Service -Name $TaskName -ErrorAction SilentlyContinue)
}

function Invoke-WinSWCommand {
  param(
    [string]$WrapperPath,
    [string[]]$Arguments
  )

  if (-not (Test-Path -LiteralPath $WrapperPath)) {
    throw "WinSW wrapper not found: $WrapperPath"
  }

  & $WrapperPath @Arguments
  $exitCode = $LASTEXITCODE
  if ($null -eq $exitCode) {
    $exitCode = 0
  }
  if ($exitCode -ne 0) {
    throw ("WinSW command failed ({0} {1}) with exit code {2}" -f $WrapperPath, ($Arguments -join " "), $exitCode)
  }
}

function Stop-ServiceRegistration {
  param(
    [string]$TaskName,
    [string]$WrapperPath
  )

  $service = Get-Service -Name $TaskName -ErrorAction SilentlyContinue
  if ($null -eq $service) {
    return
  }

  if ($service.Status -ne [System.ServiceProcess.ServiceControllerStatus]::Stopped) {
    if (Test-Path -LiteralPath $WrapperPath) {
      Invoke-WinSWCommand -WrapperPath $WrapperPath -Arguments @("stop")
    }
    else {
      Stop-Service -Name $TaskName -Force -ErrorAction SilentlyContinue
    }
  }
}

function Remove-ServiceRegistration {
  param(
    [string]$TaskName,
    [string]$WrapperPath
  )

  if (-not (Test-ServiceExists -TaskName $TaskName)) {
    return
  }

  Stop-ServiceRegistration -TaskName $TaskName -WrapperPath $WrapperPath

  if (Test-Path -LiteralPath $WrapperPath) {
    Invoke-WinSWCommand -WrapperPath $WrapperPath -Arguments @("uninstall")
    return
  }

  & sc.exe delete $TaskName | Out-Null
  if ($LASTEXITCODE -ne 0) {
    throw "failed to delete Windows service: $TaskName"
  }
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
$regionExplicit = $PSBoundParameters.ContainsKey("Region")
$envExplicit = $PSBoundParameters.ContainsKey("EnvName")
$roleExplicit = $PSBoundParameters.ContainsKey("Role")

$InstallDir = Assert-SafeInstallDir -Path $InstallDir
$configPath = Join-Path $InstallDir "agent-config.yaml"
$canPrompt = -not $NoPrompt -and (Test-CanPrompt)
$configExists = Test-Path -LiteralPath $configPath

if ($configExists) {
  if (-not $webBaseExplicit) {
    $existingMasterApiUrl = Get-ConfigValue -ConfigPath $configPath -Key "master_api_url"
    if (-not [string]::IsNullOrWhiteSpace($existingMasterApiUrl)) {
      $WebBaseUrl = $existingMasterApiUrl
    }
  }

  if (-not $ingestGrpcExplicit) {
    $existingIngestGatewayGrpcAddr = Get-ConfigValue -ConfigPath $configPath -Key "ingest_gateway_grpc_addr"
    if (-not [string]::IsNullOrWhiteSpace($existingIngestGatewayGrpcAddr)) {
      $IngestGatewayGrpcAddr = $existingIngestGatewayGrpcAddr
    }
  }

  if (-not $tenantExplicit -and [string]::IsNullOrWhiteSpace($TenantCode)) {
    $TenantCode = Get-ConfigValue -ConfigPath $configPath -Key "tenant_code"
  }

  if (-not $loopIntervalExplicit) {
    $existingLoopIntervalSec = Get-ConfigValue -ConfigPath $configPath -Key "loop_interval_sec"
    if (-not [string]::IsNullOrWhiteSpace($existingLoopIntervalSec)) {
      $parsedExistingLoopInterval = 0
      if ([int]::TryParse($existingLoopIntervalSec, [ref]$parsedExistingLoopInterval)) {
        $LoopIntervalSec = $parsedExistingLoopInterval
      }
    }
  }

  if (-not $regionExplicit) {
    $existingRegion = Get-ConfigValue -ConfigPath $configPath -Key "region"
    if (-not [string]::IsNullOrWhiteSpace($existingRegion)) {
      $Region = $existingRegion
    }
  }

  if (-not $envExplicit) {
    $existingEnvName = Get-ConfigValue -ConfigPath $configPath -Key "env"
    if (-not [string]::IsNullOrWhiteSpace($existingEnvName)) {
      $EnvName = $existingEnvName
    }
  }

  if (-not $roleExplicit) {
    $existingRole = Get-ConfigValue -ConfigPath $configPath -Key "role"
    if (-not [string]::IsNullOrWhiteSpace($existingRole)) {
      $Role = $existingRole
    }
  }
}

$installMode = Get-InstallMode -ConfigPath $configPath -CanPrompt $canPrompt -UpdateBinary $UpdateBinary.IsPresent -Reinstall $Reinstall.IsPresent

if ($installMode -eq "reinstall") {
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
}

Assert-Repo -Value $Repo
if ($installMode -eq "reinstall") {
  Assert-HttpUrl -Value $WebBaseUrl -Name "web-url"
  Assert-NonEmpty -Value $IngestGatewayGrpcAddr -Name "ingest-grpc-addr"
  Assert-PositiveInt -Value $LoopIntervalSec -Name "loop-interval-sec"
}

$arch = Resolve-Arch
$asset = "gaoming-agent_windows_${arch}.zip"
$winswAsset = Get-WinSWAssetName -Arch $arch
$releaseBaseUrl = Get-ReleaseBaseUrl -Repo $Repo -Version $Version
$winswDownloadUrl = "https://github.com/winsw/winsw/releases/download/$script:WinSWVersion/$winswAsset"
$tmpDir = Join-Path ([System.IO.Path]::GetTempPath()) ("gaoming-agent-" + [guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $tmpDir -Force | Out-Null

try {
  $zipPath = Join-Path $tmpDir $asset
  $winswPath = Join-Path $tmpDir $winswAsset
  $checksumPath = Join-Path $tmpDir "checksums.txt"
  $extractDir = Join-Path $tmpDir "extract"
  $wrapperPath = Get-ServiceWrapperExePath -InstallDir $InstallDir
  $wrapperConfigPath = Get-ServiceWrapperConfigPath -InstallDir $InstallDir
  $serviceExists = Test-ServiceExists -TaskName $TaskName

  Write-Info "downloading $asset"
  Invoke-WebRequest -Uri "$releaseBaseUrl/$asset" -OutFile $zipPath @script:WebParams
  Invoke-WebRequest -Uri "$releaseBaseUrl/checksums.txt" -OutFile $checksumPath @script:WebParams
  Write-Info "downloading $winswAsset"
  Invoke-WebRequest -Uri $winswDownloadUrl -OutFile $winswPath @script:WebParams

  $expected = Get-ExpectedSha256 -ChecksumFile $checksumPath -FileName $asset
  $actual = (Get-FileHash -Algorithm SHA256 -Path $zipPath).Hash.ToLowerInvariant()
  if ($expected -ne $actual) {
    throw "checksum mismatch for $asset"
  }

  if (Get-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue) {
    Stop-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue
    Unregister-ScheduledTask -TaskName $TaskName -Confirm:$false
  }

  $serviceAction = "install"
  if ($serviceExists) {
    if ($installMode -eq "reinstall") {
      Remove-ServiceRegistration -TaskName $TaskName -WrapperPath $wrapperPath
    }
    else {
      if (Test-Path -LiteralPath $wrapperPath) {
        Stop-ServiceRegistration -TaskName $TaskName -WrapperPath $wrapperPath
        $serviceAction = "refresh"
      }
      else {
        Remove-ServiceRegistration -TaskName $TaskName -WrapperPath $wrapperPath
      }
    }
  }

  if ($installMode -eq "reinstall" -and (Test-Path -LiteralPath $InstallDir)) {
    Remove-Item -LiteralPath $InstallDir -Recurse -Force
  }

  New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
  New-Item -ItemType Directory -Path $extractDir -Force | Out-Null
  Expand-Archive -Path $zipPath -DestinationPath $extractDir -Force

  $exePath = Join-Path $InstallDir "gaoming-agent.exe"
  $downloadedExePath = Join-Path $extractDir "gaoming-agent.exe"
  if (-not (Test-Path -LiteralPath $downloadedExePath)) {
    throw "gaoming-agent.exe not found in $extractDir"
  }

  Copy-Item -LiteralPath $downloadedExePath -Destination $exePath -Force
  Copy-Item -LiteralPath $winswPath -Destination $wrapperPath -Force
  New-Item -ItemType Directory -Path (Join-Path $InstallDir "logs") -Force | Out-Null

  if ($installMode -eq "reinstall") {
    $configBody = Render-AgentConfig `
      -MasterApiUrl $WebBaseUrl `
      -IngestGatewayGrpcAddr $IngestGatewayGrpcAddr `
      -Region $Region `
      -EnvName $EnvName `
      -Role $Role `
      -TenantCode $TenantCode `
      -LoopIntervalSec $LoopIntervalSec
    Set-Utf8NoBomContent -Path $configPath -Content $configBody
  }
  elseif (-not (Test-Path -LiteralPath $configPath)) {
    throw "installed config not found: $configPath"
  }

  $wrapperConfigBody = Render-ServiceWrapperConfig -TaskName $TaskName
  Set-Utf8NoBomContent -Path $wrapperConfigPath -Content $wrapperConfigBody

  if ($serviceAction -eq "refresh") {
    Invoke-WinSWCommand -WrapperPath $wrapperPath -Arguments @("refresh")
  }
  else {
    Invoke-WinSWCommand -WrapperPath $wrapperPath -Arguments @("install")
  }
  Invoke-WinSWCommand -WrapperPath $wrapperPath -Arguments @("start")

  $effectiveWebBaseUrl = Get-ConfigValue -ConfigPath $configPath -Key "master_api_url"
  if ([string]::IsNullOrWhiteSpace($effectiveWebBaseUrl)) {
    $effectiveWebBaseUrl = $WebBaseUrl
  }

  $effectiveIngestGatewayGrpcAddr = Get-ConfigValue -ConfigPath $configPath -Key "ingest_gateway_grpc_addr"
  if ([string]::IsNullOrWhiteSpace($effectiveIngestGatewayGrpcAddr)) {
    $effectiveIngestGatewayGrpcAddr = $IngestGatewayGrpcAddr
  }

  $effectiveTenantCode = Get-ExistingTenantCode -ConfigPath $configPath
  if ([string]::IsNullOrWhiteSpace($effectiveTenantCode)) {
    $effectiveTenantCode = Wait-ForTenantCode -ConfigPath $configPath
  }

  Write-Output "installed $TaskName to $InstallDir"
  Write-Output "install mode: $installMode"
  Write-Output "service manager: winsw"
  Write-Output "config: $configPath"
  Write-Output "wrapper: $wrapperPath"
  Write-Output "ingest_grpc_addr: $effectiveIngestGatewayGrpcAddr"
  if (-not [string]::IsNullOrWhiteSpace($effectiveTenantCode)) {
    Write-Output "tenant_code: $effectiveTenantCode"
    Write-Output ("dashboard: " + (Build-DashboardUrl -WebBaseUrl $effectiveWebBaseUrl -TenantCode $effectiveTenantCode))
    Write-Output ("hosts api: " + (Build-HostsApiUrl -WebBaseUrl $effectiveWebBaseUrl -TenantCode $effectiveTenantCode))
  }
  else {
    Write-Output "tenant_code: <pending>"
    Write-Output "tenant_code will be requested from master-api at runtime; if that fails, agent will generate one locally"
  }
}
finally {
  Remove-Item -Path $tmpDir -Recurse -Force -ErrorAction SilentlyContinue
}
