$ErrorActionPreference = 'Stop'

param(
  [string]$InstallDir = "$env:ProgramFiles\GaomingAgent",
  [string]$TaskName = "GaomingAgent"
)

function Assert-Admin {
  $principal = New-Object Security.Principal.WindowsPrincipal([Security.Principal.WindowsIdentity]::GetCurrent())
  if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    throw "uninstall-agent.ps1 must run as Administrator"
  }
}

Assert-Admin

Unregister-ScheduledTask -TaskName $TaskName -Confirm:$false -ErrorAction SilentlyContinue
Remove-Item -Recurse -Force $InstallDir -ErrorAction SilentlyContinue
Write-Host "uninstalled $TaskName"
