$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest
$installer = Join-Path $PSScriptRoot "../install-agent.ps1"
$windowsHost = [Environment]::OSVersion.Platform -eq [PlatformID]::Win32NT
$tokens = $null
$errors = $null
$ast = [System.Management.Automation.Language.Parser]::ParseFile($installer, [ref]$tokens, [ref]$errors)
if ($errors.Count) { throw ($errors | Out-String) }

# Load actual function definitions without installing services on the test host.
foreach ($function in $ast.FindAll({ param($node) $node -is [System.Management.Automation.Language.FunctionDefinitionAst] }, $false)) {
  . ([scriptblock]::Create($function.Extent.Text))
}
function Assert-Equal($Actual, $Expected) {
  if ($Actual -cne $Expected) { throw "Expected [$Expected], got [$Actual]" }
}

$defaults = & ([scriptblock]::Create($ast.ParamBlock.Extent.Text + @'

@{ WebBaseUrl = $WebBaseUrl; Ingest = $IngestGatewayGrpcAddr; Interval = $LoopIntervalSec; Interactive = $Interactive; InstallDir = $InstallDir }
'@))
Assert-Equal $defaults.WebBaseUrl "https://gm-metric.gofxq.com/"
Assert-Equal $defaults.Ingest "gm-rpc.gofxq.com:443"
Assert-Equal $defaults.Interval 5
Assert-Equal ([bool]$defaults.Interactive) $false
if ($windowsHost -and -not $defaults.InstallDir.StartsWith($env:ProgramFiles)) {
  throw "Default service directory must be under Program Files"
}

$NoPrompt = $false
$Interactive = $false
function Test-CanPrompt { throw "Default installation must not consult the terminal" }
Assert-Equal (Read-ValueWithDefault -Label test -CurrentValue existing -DisplayDefault existing) existing

$tmp = Join-Path ([IO.Path]::GetTempPath()) ("gaoming-installer-test-" + [guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory $tmp | Out-Null
try {
  $config = Join-Path $tmp "agent-config.yaml"
  Set-Content -LiteralPath $config 'tenant_code: "team"'
  Assert-Equal (Get-InstallMode -ConfigPath $config -CanPrompt $false -UpdateBinary $false -Reinstall $false) "update-binary"
  Set-Content -LiteralPath $config 'tenant_code: ""'
  Assert-Equal (Get-ExistingTenantCode -ConfigPath $config) ""
  $script:WinSWVersion = "v2.12.0"
  Set-Content -LiteralPath (Join-Path $tmp "wrapper.exe") "tampered"
  $rejected = $false
  try { Assert-WinSWChecksum -Path (Join-Path $tmp "wrapper.exe") -Asset "WinSW-x64.exe" }
  catch { $rejected = $true }
  Assert-Equal $rejected $true

  # Rendered YAML must round-trip quotes, backslashes and newlines safely.
  $body = Render-AgentConfig -MasterApiUrl 'https://example.com/' -IngestGatewayGrpcAddr 'example.com:443' -Region 'a"b\c' -EnvName "prod`nextra: injected" -Role node -TenantCode team -LoopIntervalSec 5
  if (-not $body.Contains('region: "a\"b\\c"')) { throw "YAML quotes/backslashes were not escaped" }
  if (-not $body.Contains('env: "prod\nextra: injected"')) { throw "YAML newline injection" }

  # Decode and execute the actual elevation command with a harmless target script.
  $target = Join-Path $tmp "path with spaces.ps1"
  Set-Content -LiteralPath $target 'param([string]$TenantCode, [switch]$Interactive) return @{ Tenant = $TenantCode; Interactive = [bool]$Interactive }'
  $encoded = Get-ElevationCommand -ScriptPath $target -BoundParameters @{ TenantCode = "a'b `" c; throw 'injected'"; Interactive = $true }
  $command = [Text.Encoding]::Unicode.GetString([Convert]::FromBase64String($encoded))
  $result = & ([scriptblock]::Create($command))
  Assert-Equal $result.Tenant "a'b `" c; throw 'injected'"
  Assert-Equal $result.Interactive $true

  $destination = Join-Path $tmp "agent.exe"
  $source = Join-Path $tmp "new.exe"
  Set-Content -LiteralPath $destination "old binary"
  Set-Content -LiteralPath $source "new binary"
  Install-FileAtomically -SourcePath $source -DestinationPath $destination
  Assert-Equal (Get-Content -LiteralPath $destination -Raw).Trim() "new binary"
  # A failed source copy must leave the installed executable intact.
  try { Install-FileAtomically -SourcePath (Join-Path $tmp "absent") -DestinationPath $destination } catch { }
  Assert-Equal (Get-Content -LiteralPath $destination -Raw).Trim() "new binary"

  $preserved = Join-Path $tmp "preserved.yaml"
  Set-Content -LiteralPath $config 'tenant_code: "legacy-team"'
  Copy-ExistingConfig -SourcePath $config -DestinationPath $preserved
  Assert-Equal (Get-ExistingTenantCode -ConfigPath $preserved) "legacy-team"
  Set-Content -LiteralPath $config 'tenant_code: "other-team"'
  Copy-ExistingConfig -SourcePath $config -DestinationPath $preserved
  Assert-Equal (Get-ExistingTenantCode -ConfigPath $preserved) "legacy-team"

  function Invoke-RestMethod { return @{ tenant_code = "tenant-from-server" } }
  Assert-Equal (Resolve-TenantCode -WebBaseUrl "https://example.com" -TenantCode "") "tenant-from-server"
  function Invoke-RestMethod { throw "offline" }
  $fallback = Resolve-TenantCode -WebBaseUrl "https://example.com" -TenantCode ""
  if ($fallback -notmatch '^tenant-local-[a-f0-9]{32}$') { throw "Missing offline tenant fallback" }
  Assert-Equal (Resolve-TenantCode -WebBaseUrl "https://example.com" -TenantCode team) "team"

  # Both installation and removal must reject Program Files itself, including
  # trailing separators and paths that normalize back to that parent.
  foreach ($scriptName in @("install-agent.ps1", "uninstall-agent.ps1")) {
    $scriptAst = [System.Management.Automation.Language.Parser]::ParseFile((Join-Path $PSScriptRoot "../$scriptName"), [ref]$tokens, [ref]$errors)
    if ($errors.Count) { throw ($errors | Out-String) }
    $validation = $scriptAst.Find({ param($node) $node -is [System.Management.Automation.Language.FunctionDefinitionAst] -and $node.Name -eq "Assert-SafeInstallDir" }, $false)
    . ([scriptblock]::Create($validation.Extent.Text))
    if ($windowsHost) {
      foreach ($unsafe in @($env:ProgramFiles, "$env:ProgramFiles\", "$env:ProgramFiles\GaomingAgent\..\")) {
        $rejected = $false
        try { Assert-SafeInstallDir -Path $unsafe | Out-Null } catch { $rejected = $true }
        Assert-Equal $rejected $true
      }
    }
  }

  if ($windowsHost -and (Test-IsAdministrator)) {
    $secured = Join-Path $tmp "secured"
    New-Item -ItemType Directory $secured | Out-Null
    Set-Content -LiteralPath (Join-Path $secured "agent.exe") "fixture"
    Set-InstallDirectoryPermissions -Path $secured
    $acl = Get-Acl -LiteralPath (Join-Path $secured "agent.exe")
    foreach ($entry in $acl.Access) {
      $sid = $entry.IdentityReference.Translate([Security.Principal.SecurityIdentifier]).Value
      if ($sid -notin @("S-1-5-18", "S-1-5-32-544")) { throw "Service files accessible by unexpected identity: $sid" }
    }
  }

  Write-Host "PowerShell installer tests passed"
}
finally { Remove-Item -LiteralPath $tmp -Recurse -Force }
