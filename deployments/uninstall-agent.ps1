$ErrorActionPreference = 'Stop'
$script:BootstrapLogPath = Join-Path ([System.IO.Path]::GetTempPath()) "gaoming-uninstall-agent.log"

param(
  [string]$InstallDir = "$env:LOCALAPPDATA\GaomingAgent",
  [string]$TaskName = "GaomingAgent"
)

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

  $tempPath = Join-Path ([System.IO.Path]::GetTempPath()) ("gaoming-uninstall-agent-elevated-" + [guid]::NewGuid().ToString("N") + ".ps1")
  $encoding = New-Object System.Text.UTF8Encoding($false)
  [System.IO.File]::WriteAllText($tempPath, $definition, $encoding)
  Write-BootstrapLog ("materialized elevated script to: " + $tempPath)
  return $tempPath
}

function Ensure-Elevated {
  param([hashtable]$BoundParameters)

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
    $args += "-" + $entry.Key
    $args += [string]$entry.Value
  }

  $argumentLine = ($args | ForEach-Object { ConvertTo-PowerShellLiteral -Value $_ }) -join " "
  Write-BootstrapLog ("requesting UAC elevation; log file: " + $script:BootstrapLogPath)
  Start-Process -FilePath (Get-CurrentPowerShellExe) -Verb RunAs -ArgumentList $argumentLine | Out-Null
  exit 0
}

Ensure-Elevated -BoundParameters $PSBoundParameters

function Get-ServiceWrapperExePath {
  param([string]$InstallDir)

  return Join-Path $InstallDir "gaoming-agent-service.exe"
}

$wrapperPath = Get-ServiceWrapperExePath -InstallDir $InstallDir

if (Get-Service -Name $TaskName -ErrorAction SilentlyContinue) {
  if (Test-Path -LiteralPath $wrapperPath) {
    & $wrapperPath stop | Out-Null
    & $wrapperPath uninstall | Out-Null
    if ($LASTEXITCODE -ne 0) {
      throw "failed to uninstall Windows service via WinSW: $TaskName"
    }
  }
  else {
    Stop-Service -Name $TaskName -Force -ErrorAction SilentlyContinue
    & sc.exe delete $TaskName | Out-Null
    if ($LASTEXITCODE -ne 0) {
      throw "failed to delete Windows service: $TaskName"
    }
  }
}

Unregister-ScheduledTask -TaskName $TaskName -Confirm:$false -ErrorAction SilentlyContinue
Remove-Item -Recurse -Force $InstallDir -ErrorAction SilentlyContinue
Write-Host "uninstalled $TaskName"
