[CmdletBinding()]
param(
  [ValidateRange(1024, 65535)]
  [int]$CdpPort = 19226,
  [ValidateRange(5, 180)]
  [int]$StartupTimeoutSeconds = 45
)

$ErrorActionPreference = "Stop"
$root = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
$frontend = Join-Path $root "frontend"
$evidencePath = Join-Path $root ".planning\phases\13-unified-ui-design-system-and-automated-enforcement\evidence\dialog-feasibility.json"
$process = $null
$userDataDir = $null
$previousArgs = $null
$previousUserData = $null
$startedAt = (Get-Date).ToUniversalTime().ToString("o")
$evidence = [ordered]@{
  schema_version = 1
  proof = "dialog-feasibility"
  status = "failed"
  started_at = $startedAt
  completed_at = $null
  build = [ordered]@{ command = "mage Build"; executable = $null; sha256 = $null }
  runtime = [ordered]@{ cdp_endpoint = "http://127.0.0.1:$CdpPort"; browser = $null; user_data_dir = $null }
  test = [ordered]@{ command = "npx playwright test e2e/dialog-feasibility.spec.ts --project=chromium --workers=1"; exit_code = $null }
  error = $null
}

function Write-Evidence {
  $evidence.completed_at = (Get-Date).ToUniversalTime().ToString("o")
  New-Item -ItemType Directory -Force -Path (Split-Path -Parent $evidencePath) | Out-Null
  $json = ($evidence | ConvertTo-Json -Depth 6) + "`n"
  [System.IO.File]::WriteAllText($evidencePath, $json, [System.Text.UTF8Encoding]::new($false))
}

try {
  $mage = Get-Command mage -ErrorAction Stop
  & $mage.Source Build
  if ($LASTEXITCODE -ne 0) { throw "GOLC_DIALOG_PROOF_BUILD_FAILED: mage Build exited $LASTEXITCODE" }

  $executable = Join-Path $root "golc-desktop.exe"
  if (-not (Test-Path -LiteralPath $executable -PathType Leaf)) {
    throw "GOLC_DIALOG_PROOF_BINARY_MISSING: mage Build did not produce $executable"
  }
  $evidence.build.executable = $executable
  $evidence.build.sha256 = (Get-FileHash -LiteralPath $executable -Algorithm SHA256).Hash

  # WebView2 accepts this documented host environment variable at process
  # startup. Preserve the caller's value so the proof never contaminates a
  # developer shell or terminates a process it did not create.
  $previousArgs = $env:WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS
  $previousUserData = $env:WEBVIEW2_USER_DATA_FOLDER
  $previousPath = $env:Path
  $env:WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS = "--remote-debugging-port=$CdpPort"
  $userDataDir = Join-Path ([System.IO.Path]::GetTempPath()) "golc-dialog-proof-$PID-$CdpPort"
  New-Item -ItemType Directory -Force -Path $userDataDir | Out-Null
  $env:WEBVIEW2_USER_DATA_FOLDER = $userDataDir
  $evidence.runtime.user_data_dir = $userDataDir
  $goBin = Join-Path $root ".tools\cache\go-bin"
  if (Test-Path -LiteralPath $goBin -PathType Container) { $env:Path = "$goBin;$env:Path" }
  $process = Start-Process -FilePath $executable -WorkingDirectory $root -PassThru

  $deadline = (Get-Date).AddSeconds($StartupTimeoutSeconds)
  $version = $null
  while ((Get-Date) -lt $deadline) {
    if ($process.HasExited) { throw "GOLC_DIALOG_PROOF_APP_EXITED: spawned process exited with code $($process.ExitCode)" }
    try {
      $version = Invoke-RestMethod -Uri "http://127.0.0.1:$CdpPort/json/version" -TimeoutSec 2
      if ($version) { break }
    } catch {
      Start-Sleep -Milliseconds 250
    }
  }
  if (-not $version) { throw "GOLC_DIALOG_PROOF_CDP_TIMEOUT: WebView2 CDP endpoint did not become ready within $StartupTimeoutSeconds seconds" }
  $evidence.runtime.browser = $version.Browser

  Push-Location $frontend
  try {
    $env:GOLC_WEBVIEW2_CDP_ENDPOINT = "http://127.0.0.1:$CdpPort"
    & npx playwright test e2e/dialog-feasibility.spec.ts --project=chromium --workers=1
    $evidence.test.exit_code = $LASTEXITCODE
    if ($LASTEXITCODE -ne 0) { throw "GOLC_DIALOG_PROOF_TEST_FAILED: Playwright exited $LASTEXITCODE" }
  } finally {
    Pop-Location
  }

  $evidence.status = "passed"
} catch {
  $evidence.error = $_.Exception.Message
  throw
} finally {
  if ($null -ne $process -and -not $process.HasExited) {
    Stop-Process -Id $process.Id -Force
  }
  if ($null -ne $previousArgs) { $env:WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS = $previousArgs } else { Remove-Item Env:WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS -ErrorAction SilentlyContinue }
  if ($null -ne $previousUserData) { $env:WEBVIEW2_USER_DATA_FOLDER = $previousUserData } else { Remove-Item Env:WEBVIEW2_USER_DATA_FOLDER -ErrorAction SilentlyContinue }
  if ($null -ne $previousPath) { $env:Path = $previousPath }
  if ($null -ne $userDataDir -and (Test-Path -LiteralPath $userDataDir -PathType Container)) { Remove-Item -LiteralPath $userDataDir -Recurse -Force }
  Write-Evidence
}
