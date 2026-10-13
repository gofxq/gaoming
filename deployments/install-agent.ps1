[CmdletBinding()]
param(
  [ValidateNotNullOrEmpty()][string]$Repo = "gofxq/gaoming",
  [ValidateNotNullOrEmpty()][string]$Version = "latest",
  [ValidateNotNullOrEmpty()][string]$InstallDir = "$env:ProgramFiles\GaomingAgent",
  [ValidatePattern('^[A-Za-z0-9._-]+$')][string]$TaskName = "GaomingAgent",
  [Alias("MasterApiUrl")][ValidateNotNullOrEmpty()][string]$WebBaseUrl = "https://gm-metric.gofxq.com/",
  [ValidateNotNullOrEmpty()][string]$IngestGatewayGrpcAddr = "gm-rpc.gofxq.com:443",
  [string]$TenantCode = "",
  [ValidateRange(1, 3600)][int]$LoopIntervalSec = 5,
  [ValidateNotNullOrEmpty()][string]$Region = "local",
  [ValidateNotNullOrEmpty()][string]$EnvName = "prod",
  [ValidateNotNullOrEmpty()][string]$Role = "node",
  [switch]$UpdateBinary,
  [switch]$Reinstall,
  [switch]$Interactive,
  [switch]$NoPrompt
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

$script:WebParams = @{ TimeoutSec = 60 }
$script:WinSWVersion = "v2.12.0"
$script:InstallerPath = $PSCommandPath
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
  if (-not [string]::IsNullOrWhiteSpace($script:InstallerPath)) {
    return $script:InstallerPath
  }
  throw "Download install-agent.ps1 to a file and execute it; piping to iex does not support elevation"
}

function Get-ElevationCommand {
  param([string]$ScriptPath, [hashtable]$BoundParameters)

  $command = "& " + (ConvertTo-PowerShellLiteral -Value $ScriptPath)
  foreach ($entry in $BoundParameters.GetEnumerator()) {
    if ($entry.Key -notmatch '^[A-Za-z][A-Za-z0-9]*$') { throw "Invalid parameter name" }
    if ($entry.Value -is [System.Management.Automation.SwitchParameter] -or $entry.Value -is [bool]) {
      $command += " -" + $entry.Key + ":" + ('$' + ([bool]$entry.Value).ToString().ToLowerInvariant())
    }
    else {
      $command += " -" + $entry.Key + " " + (ConvertTo-PowerShellLiteral -Value $entry.Value)
    }
  }
  return [Convert]::ToBase64String([Text.Encoding]::Unicode.GetBytes($command))
}

function Ensure-Elevated {
  param(
    [hashtable]$BoundParameters
  )

  if (Test-IsAdministrator) {
    Write-BootstrapLog "running with administrator privileges"
    return
  }

  $relaunchPath = Get-ScriptRelaunchPath
  $encoded = Get-ElevationCommand -ScriptPath $relaunchPath -BoundParameters $BoundParameters
  Write-BootstrapLog ("requesting UAC elevation; log file: " + $script:BootstrapLogPath)
  $child = Start-Process -FilePath (Get-CurrentPowerShellExe) -Verb RunAs -ArgumentList @("-NoProfile", "-ExecutionPolicy", "Bypass", "-EncodedCommand", $encoded) -Wait -PassThru
  if ($child.ExitCode -ne 0) { throw "Elevated installer failed with exit code $($child.ExitCode)" }
  exit $child.ExitCode
}

Ensure-Elevated -BoundParameters $PSBoundParameters

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

  if (-not $Interactive -or $NoPrompt -or -not (Test-CanPrompt)) {
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
  $programFiles = [System.IO.Path]::GetFullPath($env:ProgramFiles).TrimEnd('\') + '\'
  if ([string]::Equals($full.TrimEnd('\'), $programFiles.TrimEnd('\'), [StringComparison]::OrdinalIgnoreCase) -or -not $full.StartsWith($programFiles, [StringComparison]::OrdinalIgnoreCase)) {
    throw "InstallDir must be a dedicated directory under Program Files: $full"
  }
  $itemPath = $full
  while ($itemPath) {
    if ((Test-Path -LiteralPath $itemPath) -and ((Get-Item -LiteralPath $itemPath -Force).Attributes -band [IO.FileAttributes]::ReparsePoint)) {
      throw "InstallDir must not contain junctions or symbolic links: $itemPath"
    }
    $itemPath = Split-Path -Parent $itemPath
  }

  return $full
}

function Set-InstallDirectoryPermissions {
  param([string]$Path)
  if (Get-ChildItem -LiteralPath $Path -Recurse -Force | Where-Object { $_.Attributes -band [IO.FileAttributes]::ReparsePoint }) {
    throw "Installed files must not contain junctions or symbolic links"
  }
  $acl = New-Object Security.AccessControl.DirectorySecurity
  $acl.SetAccessRuleProtection($true, $false)
  foreach ($sid in @("S-1-5-18", "S-1-5-32-544")) {
    $identity = New-Object Security.Principal.SecurityIdentifier($sid)
    $rule = New-Object Security.AccessControl.FileSystemAccessRule($identity, "FullControl", "ContainerInherit,ObjectInherit", "None", "Allow")
    $acl.AddAccessRule($rule)
  }
  $acl.SetOwner((New-Object Security.Principal.SecurityIdentifier("S-1-5-32-544")))
  Set-Acl -LiteralPath $Path -AclObject $acl
  & icacls.exe $Path /setowner '*S-1-5-32-544' /T /Q | Out-Null
  if ($LASTEXITCODE -ne 0) { throw "Failed to secure installed file ownership" }
  # Remove any explicit grants retained from older installations.
  foreach ($child in Get-ChildItem -LiteralPath $Path -Force) {
    & icacls.exe $child.FullName /reset /T /Q | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "Failed to secure installed files" }
  }
}

function Copy-ExistingConfig {
  param([string]$SourcePath, [string]$DestinationPath)
  if (-not (Test-Path -LiteralPath $DestinationPath) -and (Test-Path -LiteralPath $SourcePath)) {
    Install-FileAtomically -SourcePath $SourcePath -DestinationPath $DestinationPath
  }
}

function Resolve-TenantCode {
  param([string]$WebBaseUrl, [string]$TenantCode)
  if (-not [string]::IsNullOrWhiteSpace($TenantCode)) { return $TenantCode }
  $base = $WebBaseUrl.TrimEnd('/')
  if ($base.EndsWith('/master')) { $base += '/api/v1' }
  elseif (-not $base.EndsWith('/master/api/v1')) { $base += '/master/api/v1' }
  try {
    $response = Invoke-RestMethod -Method Post -Uri "$base/install/tenant" -TimeoutSec 15
    if (-not [string]::IsNullOrWhiteSpace($response.tenant_code)) { return [string]$response.tenant_code }
  }
  catch { Write-Warning "Could not allocate a tenant from the server; generating a local tenant" }
  return "tenant-local-" + [guid]::NewGuid().ToString("N")
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

function Assert-WinSWChecksum {
  param([string]$Path, [string]$Asset)
  # SHA256 of the official v2.12.0 release assets, checked before execution.
  $hashes = @{
    "WinSW-x64.exe" = "05b82d46ad331cc16bdc00de5c6332c1ef818df8ceefcd49c726553209b3a0da"
    "WinSW.NET461.exe" = "b5066b7bbdfba1293e5d15cda3caaea88fbeab35bd5b38c41c913d492aadfc4f"
  }
  if (-not $hashes.ContainsKey($Asset) -or (Get-FileHash -Algorithm SHA256 -LiteralPath $Path).Hash.ToLowerInvariant() -ne $hashes[$Asset]) {
    throw "WinSW checksum mismatch: $Asset"
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

  return Get-ConfigValue -ConfigPath $ConfigPath -Key "tenant_code"
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
  $pattern = '^\s*' + $escapedKey + ':\s*(?<value>.*?)\s*$'
  $match = Select-String -Path $ConfigPath -Pattern $pattern | Select-Object -First 1
  if ($match) {
    $value = $match.Matches[0].Groups['value'].Value.Trim()
    if ($value.StartsWith('"') -and $value.EndsWith('"')) {
      try { return ConvertFrom-Json -InputObject $value } catch { return $value.Trim('"') }
    }
    return $value.Trim("'")
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
  <onfailure action="restart" delay="5 sec" />
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
master_api_url: $(ConvertTo-Json -InputObject $MasterApiUrl -Compress)
ingest_gateway_grpc_addr: $(ConvertTo-Json -InputObject $IngestGatewayGrpcAddr -Compress)
region: $(ConvertTo-Json -InputObject $Region -Compress)
env: $(ConvertTo-Json -InputObject $EnvName -Compress)
role: $(ConvertTo-Json -InputObject $Role -Compress)
tenant_code: $(ConvertTo-Json -InputObject $TenantCode -Compress)
loop_interval_sec: $LoopIntervalSec
"@
}

function Set-Utf8NoBomContent {
  param(
    [string]$Path,
    [string]$Content
  )

  $temporary = $Path + ".text-" + [guid]::NewGuid().ToString("N")
  try {
    $encoding = New-Object System.Text.UTF8Encoding($false)
    [System.IO.File]::WriteAllText($temporary, $Content, $encoding)
    Install-FileAtomically -SourcePath $temporary -DestinationPath $Path
  }
  finally { if (Test-Path -LiteralPath $temporary) { Remove-Item -LiteralPath $temporary -Force } }
}

function Install-FileAtomically {
  param([string]$SourcePath, [string]$DestinationPath)
  $staged = $DestinationPath + ".new-" + [guid]::NewGuid().ToString("N")
  try {
    Copy-Item -LiteralPath $SourcePath -Destination $staged
    if (Test-Path -LiteralPath $DestinationPath) {
      [IO.File]::Replace($staged, $DestinationPath, [System.Management.Automation.Language.NullString]::Value)
    }
    else { [IO.File]::Move($staged, $DestinationPath) }
  }
  finally { if (Test-Path -LiteralPath $staged) { Remove-Item -LiteralPath $staged -Force } }
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

  # Use the SCM directly when replacing a legacy service; do not execute its
  # wrapper from the old user-writable installation directory as administrator.
  $service = Get-Service -Name $TaskName
  if ($service.Status -ne [System.ServiceProcess.ServiceControllerStatus]::Stopped) {
    Stop-Service -Name $TaskName -Force
    $service.WaitForStatus([System.ServiceProcess.ServiceControllerStatus]::Stopped, [TimeSpan]::FromSeconds(30))
  }
  & sc.exe delete $TaskName | Out-Null
  if ($LASTEXITCODE -ne 0) {
    throw "failed to delete Windows service: $TaskName"
  }
  $service.Dispose()
  for ($attempt = 0; $attempt -lt 30; $attempt++) {
    if (-not (Test-ServiceExists -TaskName $TaskName)) { return }
    Start-Sleep -Seconds 1
  }
  throw "Timed out deleting Windows service: $TaskName"
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
$configSourcePath = $configPath
$legacyConfigPath = Join-Path $env:LOCALAPPDATA "GaomingAgent\agent-config.yaml"
if (-not $PSBoundParameters.ContainsKey("InstallDir") -and -not (Test-Path -LiteralPath $configPath) -and (Test-Path -LiteralPath $legacyConfigPath)) {
  $configSourcePath = $legacyConfigPath
  Write-Info "Migrating legacy configuration to Program Files"
}
$canPrompt = $Interactive -and -not $NoPrompt -and (Test-CanPrompt)
$configExists = Test-Path -LiteralPath $configSourcePath

if ($configExists) {
  if (-not $webBaseExplicit) {
    $existingMasterApiUrl = Get-ConfigValue -ConfigPath $configSourcePath -Key "master_api_url"
    if (-not [string]::IsNullOrWhiteSpace($existingMasterApiUrl)) {
      $WebBaseUrl = $existingMasterApiUrl
    }
  }

  if (-not $ingestGrpcExplicit) {
    $existingIngestGatewayGrpcAddr = Get-ConfigValue -ConfigPath $configSourcePath -Key "ingest_gateway_grpc_addr"
    if (-not [string]::IsNullOrWhiteSpace($existingIngestGatewayGrpcAddr)) {
      $IngestGatewayGrpcAddr = $existingIngestGatewayGrpcAddr
    }
  }

  if (-not $tenantExplicit -and [string]::IsNullOrWhiteSpace($TenantCode)) {
    $TenantCode = Get-ConfigValue -ConfigPath $configSourcePath -Key "tenant_code"
  }

  if (-not $loopIntervalExplicit) {
    $existingLoopIntervalSec = Get-ConfigValue -ConfigPath $configSourcePath -Key "loop_interval_sec"
    if (-not [string]::IsNullOrWhiteSpace($existingLoopIntervalSec)) {
      $parsedExistingLoopInterval = 0
      if ([int]::TryParse($existingLoopIntervalSec, [ref]$parsedExistingLoopInterval)) {
        $LoopIntervalSec = $parsedExistingLoopInterval
      }
    }
  }

  if (-not $regionExplicit) {
    $existingRegion = Get-ConfigValue -ConfigPath $configSourcePath -Key "region"
    if (-not [string]::IsNullOrWhiteSpace($existingRegion)) {
      $Region = $existingRegion
    }
  }

  if (-not $envExplicit) {
    $existingEnvName = Get-ConfigValue -ConfigPath $configSourcePath -Key "env"
    if (-not [string]::IsNullOrWhiteSpace($existingEnvName)) {
      $EnvName = $existingEnvName
    }
  }

  if (-not $roleExplicit) {
    $existingRole = Get-ConfigValue -ConfigPath $configSourcePath -Key "role"
    if (-not [string]::IsNullOrWhiteSpace($existingRole)) {
      $Role = $existingRole
    }
  }
}

$installMode = Get-InstallMode -ConfigPath $configSourcePath -CanPrompt $canPrompt -UpdateBinary $UpdateBinary.IsPresent -Reinstall $Reinstall.IsPresent

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
  $TenantCode = Resolve-TenantCode -WebBaseUrl $WebBaseUrl -TenantCode $TenantCode
}

$arch = Resolve-Arch
$asset = "gaoming-agent_windows_${arch}.zip"
$winswAsset = Get-WinSWAssetName -Arch $arch
$releaseBaseUrl = Get-ReleaseBaseUrl -Repo $Repo -Version $Version
$winswDownloadUrl = "https://github.com/winsw/winsw/releases/download/$script:WinSWVersion/$winswAsset"
$tmpDir = Join-Path ([System.IO.Path]::GetTempPath()) ("gaoming-agent-" + [guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $tmpDir -Force | Out-Null
$mutationStarted = $false
$previousFiles = @{}
$previousServicePath = ""

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
  Assert-WinSWChecksum -Path $winswPath -Asset $winswAsset

  New-Item -ItemType Directory -Path $extractDir -Force | Out-Null
  Expand-Archive -LiteralPath $zipPath -DestinationPath $extractDir -Force
  $downloadedExePath = Join-Path $extractDir "gaoming-agent.exe"
  if (-not (Test-Path -LiteralPath $downloadedExePath)) {
    throw "gaoming-agent.exe not found in $extractDir"
  }
  New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
  Set-InstallDirectoryPermissions -Path $InstallDir
  foreach ($name in @("gaoming-agent.exe", "gaoming-agent-service.exe", "gaoming-agent-service.xml", "agent-config.yaml")) {
    $installedPath = Join-Path $InstallDir $name
    if (Test-Path -LiteralPath $installedPath) {
      $backupPath = Join-Path $tmpDir ("previous-" + $name)
      Copy-Item -LiteralPath $installedPath -Destination $backupPath
      $previousFiles[$installedPath] = $backupPath
    }
  }
  if ($serviceExists) {
    $previousServicePath = (Get-CimInstance -ClassName Win32_Service -Filter "Name='$TaskName'").PathName
  }
  $mutationStarted = $true

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
      if ((Test-Path -LiteralPath $wrapperPath) -and ($previousServicePath.Trim('"') -eq $wrapperPath)) {
        Stop-ServiceRegistration -TaskName $TaskName -WrapperPath $wrapperPath
        $serviceAction = "refresh"
      }
      else {
        Remove-ServiceRegistration -TaskName $TaskName -WrapperPath $wrapperPath
      }
    }
  }

  $exePath = Join-Path $InstallDir "gaoming-agent.exe"

  Install-FileAtomically -SourcePath $downloadedExePath -DestinationPath $exePath
  Install-FileAtomically -SourcePath $winswPath -DestinationPath $wrapperPath
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
  else {
    Copy-ExistingConfig -SourcePath $configSourcePath -DestinationPath $configPath
    if (-not (Test-Path -LiteralPath $configPath)) { throw "installed config not found: $configPath" }
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
  $service = Get-Service -Name $TaskName
  $service.WaitForStatus([System.ServiceProcess.ServiceControllerStatus]::Running, [TimeSpan]::FromSeconds(30))
  $mutationStarted = $false

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
catch {
  $failure = $_
  if ($mutationStarted -and -not [string]::IsNullOrWhiteSpace($previousServicePath)) {
    Write-Warning "Installation failed; restoring previous files and service"
    try {
      Remove-ServiceRegistration -TaskName $TaskName -WrapperPath $wrapperPath
      foreach ($installedPath in $previousFiles.Keys) {
        Install-FileAtomically -SourcePath $previousFiles[$installedPath] -DestinationPath $installedPath
      }
      $restored = Invoke-CimMethod -ClassName Win32_Service -MethodName Create -Arguments @{
        Name = $TaskName; DisplayName = $TaskName; PathName = $previousServicePath
        ServiceType = [uint32]16; ErrorControl = [uint32]1; StartMode = "Automatic"
        StartName = "LocalSystem"
      }
      if ($restored.ReturnValue -ne 0) { throw "Could not restore service registration" }
      Start-Service -Name $TaskName
    }
    catch { Write-Warning "Rollback failed: $_" }
  }
  throw $failure
}
finally {
  Remove-Item -Path $tmpDir -Recurse -Force -ErrorAction SilentlyContinue
}
