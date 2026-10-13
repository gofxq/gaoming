[CmdletBinding()]
param(
  [string]$InstallDir = "$env:ProgramFiles\GaomingAgent",
  [ValidatePattern('^[A-Za-z0-9._-]+$')][string]$TaskName = "GaomingAgent"
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest
$script:InstallerPath = $PSCommandPath
function Write-BootstrapLog { param([string]$Message) Write-Host $Message }

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
  throw "Download uninstall-agent.ps1 to a file and execute it; piping to iex does not support elevation"
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
  Write-BootstrapLog "requesting UAC elevation"
  $child = Start-Process -FilePath (Get-CurrentPowerShellExe) -Verb RunAs -ArgumentList @("-NoProfile", "-ExecutionPolicy", "Bypass", "-EncodedCommand", $encoded) -Wait -PassThru
  if ($child.ExitCode -ne 0) { throw "Elevated uninstaller failed with exit code $($child.ExitCode)" }
  exit $child.ExitCode
}

Ensure-Elevated -BoundParameters $PSBoundParameters


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

$full = Assert-SafeInstallDir -Path $InstallDir
if ((Test-Path -LiteralPath $full) -and (Get-ChildItem -LiteralPath $full -Recurse -Force | Where-Object { $_.Attributes -band [IO.FileAttributes]::ReparsePoint })) {
  throw "Installed files must not contain junctions or symbolic links"
}
$service = Get-Service -Name $TaskName -ErrorAction SilentlyContinue
if ($null -ne $service) {
  if ($service.Status -ne [ServiceProcess.ServiceControllerStatus]::Stopped) {
    Stop-Service -Name $TaskName -Force
    $service.WaitForStatus([ServiceProcess.ServiceControllerStatus]::Stopped, [TimeSpan]::FromSeconds(30))
  }
  & sc.exe delete $TaskName | Out-Null
  if ($LASTEXITCODE -ne 0) { throw "Failed to delete service: $TaskName" }
  $service.Dispose()
}
Unregister-ScheduledTask -TaskName $TaskName -Confirm:$false -ErrorAction SilentlyContinue
if (Test-Path -LiteralPath $full) { Remove-Item -LiteralPath $full -Recurse -Force }
Write-Host "uninstalled $TaskName"
