[CmdletBinding()]
param(
  [ValidateRange(1024, 65535)]
  [int]$CdpPort = 19226,
  [ValidateRange(5, 180)]
  [int]$StartupTimeoutSeconds = 45,
  [string]$EvidencePath = ""
)

$ErrorActionPreference = "Stop"
$root = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
$frontend = Join-Path $root "frontend"
$evidencePath = if ([string]::IsNullOrWhiteSpace($EvidencePath)) {
  Join-Path $root ".planning\phases\13-unified-ui-design-system-and-automated-enforcement\evidence\dialog-feasibility.json"
} else {
  [System.IO.Path]::GetFullPath($EvidencePath)
}
$process = $null
$userDataDir = $null
$overlayWorkspace = $null
$overlayPath = $null
$previousOverlay = $null
$previousModfile = $null
$previousCdpPort = $null
$previousProofUserData = $null
$previousEndpoint = $null
$startedAt = (Get-Date).ToUniversalTime().ToString("o")
$evidence = [ordered]@{
  schema_version = 1
  proof = "dialog-feasibility"
  status = "failed"
  started_at = $startedAt
  completed_at = $null
  build = [ordered]@{ command = "mage Build"; executable = $null; sha256 = $null }
  runtime = [ordered]@{ cdp_endpoint = "http://127.0.0.1:$CdpPort"; browser = $null; user_data_dir = $null }
  test = [ordered]@{ command = "pinned node + local @playwright/test/cli.js test e2e/dialog-feasibility.spec.ts --project=chromium --workers=1"; exit_code = $null }
  error = $null
}

function Write-Evidence {
  $evidence.completed_at = (Get-Date).ToUniversalTime().ToString("o")
  New-Item -ItemType Directory -Force -Path (Split-Path -Parent $evidencePath) | Out-Null
  $json = ($evidence | ConvertTo-Json -Depth 6) + "`n"
  [System.IO.File]::WriteAllText($evidencePath, $json, [System.Text.UTF8Encoding]::new($false))
}

try {
  $moduleRoot = Join-Path $root ".tools\cache\go-mod\github.com\wailsapp\go-webview2@v1.0.22"
  $moduleSource = Join-Path $moduleRoot "pkg\edge\create_env_go.go"
  $overlaySource = Join-Path $root "scripts\ci\webview2-cdp-overlay\create_env_go.go"
  if (-not (Test-Path -LiteralPath $moduleSource -PathType Leaf)) {
    throw "GOLC_DIALOG_PROOF_UPSTREAM_MISSING: run mage Bootstrap before the packaged proof"
  }
  $expectedUpstreamHash = "C56B55D9050F28B53F8DEE0E6AEE6F830E09FA7AD4CF0A4DEF303273FBEBF1B2"
  $actualUpstreamHash = (Get-FileHash -LiteralPath $moduleSource -Algorithm SHA256).Hash
  if ($actualUpstreamHash -ne $expectedUpstreamHash) {
    throw "GOLC_DIALOG_PROOF_UPSTREAM_MISMATCH: go-webview2 create_env_go.go changed; review the test overlay before updating its hash"
  }
  $overlayWorkspace = Join-Path ([System.IO.Path]::GetTempPath()) "golc-dialog-proof-overlay-$PID"
  $localModuleRoot = Join-Path $overlayWorkspace "go-webview2"
  New-Item -ItemType Directory -Force -Path $overlayWorkspace | Out-Null
  Copy-Item -LiteralPath $moduleRoot -Destination $localModuleRoot -Recurse

  $modfilePath = Join-Path $overlayWorkspace "golc-dialog-proof.mod"
  $modsumPath = Join-Path $overlayWorkspace "golc-dialog-proof.sum"
  $modulePathForGo = $localModuleRoot.Replace("\", "/")
  $modfile = [System.IO.File]::ReadAllText((Join-Path $root "go.mod")).TrimEnd() +
    "`r`n`r`nreplace github.com/wailsapp/go-webview2 => $modulePathForGo`r`n"
  [System.IO.File]::WriteAllText($modfilePath, $modfile, [System.Text.UTF8Encoding]::new($false))
  Copy-Item -LiteralPath (Join-Path $root "go.sum") -Destination $modsumPath

  $localModuleSource = Join-Path $localModuleRoot "pkg\edge\create_env_go.go"
  $overlayPath = Join-Path $overlayWorkspace "overlay.json"
  $overlay = @{ Replace = @{ $localModuleSource = $overlaySource } } | ConvertTo-Json -Depth 3
  [System.IO.File]::WriteAllText($overlayPath, $overlay, [System.Text.UTF8Encoding]::new($false))
  $previousOverlay = $env:GOLC_DIALOG_PROOF_GO_OVERLAY
  $previousModfile = $env:GOLC_DIALOG_PROOF_GO_MODFILE
  $env:GOLC_DIALOG_PROOF_GO_OVERLAY = $overlayPath
  $env:GOLC_DIALOG_PROOF_GO_MODFILE = $modfilePath

  $mage = Get-Command mage -ErrorAction Stop
  & $mage.Source Build
  if ($LASTEXITCODE -ne 0) { throw "GOLC_DIALOG_PROOF_BUILD_FAILED: mage Build exited $LASTEXITCODE" }

  $executable = Join-Path $root "golc-desktop.exe"
  if (-not (Test-Path -LiteralPath $executable -PathType Leaf)) {
    throw "GOLC_DIALOG_PROOF_BINARY_MISSING: mage Build did not produce $executable"
  }
  $evidence.build.executable = $executable
  $evidence.build.sha256 = (Get-FileHash -LiteralPath $executable -Algorithm SHA256).Hash

  # The proof build's version-locked Go overlay translates these GOLC-only
  # inputs into WebView2 environment options. Ordinary builds never compile
  # that adapter, and an unset/invalid port preserves Wails' values exactly.
  $previousCdpPort = $env:GOLC_DIALOG_PROOF_CDP_PORT
  $previousProofUserData = $env:GOLC_DIALOG_PROOF_USER_DATA_FOLDER
  $previousPath = $env:Path
  $userDataDir = Join-Path ([System.IO.Path]::GetTempPath()) "golc-dialog-proof-$PID-$CdpPort"
  New-Item -ItemType Directory -Force -Path $userDataDir | Out-Null
  $env:GOLC_DIALOG_PROOF_CDP_PORT = "$CdpPort"
  $env:GOLC_DIALOG_PROOF_USER_DATA_FOLDER = $userDataDir
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

  $toolchainConfig = [System.IO.File]::ReadAllText((Join-Path $root "config\toolchain.toml"))
  $nodeVersionMatch = [regex]::Match($toolchainConfig, '(?ms)^\[toolchain\.node\]\r?\nversion\s*=\s*"([^"]+)"')
  if (-not $nodeVersionMatch.Success) {
    throw "GOLC_DIALOG_PROOF_NODE_MISSING: config/toolchain.toml does not pin toolchain.node.version"
  }
  $nodeVersion = $nodeVersionMatch.Groups[1].Value
  $nodeExecutable = Join-Path $root ".tools\toolchains\node\$nodeVersion\windows-amd64\node-v$nodeVersion-win-x64\node.exe"
  $playwrightCli = Join-Path $frontend "node_modules\@playwright\test\cli.js"
  if (-not (Test-Path -LiteralPath $nodeExecutable -PathType Leaf) -or -not (Test-Path -LiteralPath $playwrightCli -PathType Leaf)) {
    throw "GOLC_DIALOG_PROOF_NODE_MISSING: run mage Bootstrap before the packaged proof"
  }

  Push-Location $frontend
  try {
    $previousEndpoint = $env:GOLC_WEBVIEW2_CDP_ENDPOINT
    $env:GOLC_WEBVIEW2_CDP_ENDPOINT = "http://127.0.0.1:$CdpPort"
    & $nodeExecutable $playwrightCli test e2e/dialog-feasibility.spec.ts --project=chromium --workers=1
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
    $process.WaitForExit(5000) | Out-Null
  }
  if ($null -ne $previousOverlay) { $env:GOLC_DIALOG_PROOF_GO_OVERLAY = $previousOverlay } else { Remove-Item Env:GOLC_DIALOG_PROOF_GO_OVERLAY -ErrorAction SilentlyContinue }
  if ($null -ne $previousModfile) { $env:GOLC_DIALOG_PROOF_GO_MODFILE = $previousModfile } else { Remove-Item Env:GOLC_DIALOG_PROOF_GO_MODFILE -ErrorAction SilentlyContinue }
  if ($null -ne $previousCdpPort) { $env:GOLC_DIALOG_PROOF_CDP_PORT = $previousCdpPort } else { Remove-Item Env:GOLC_DIALOG_PROOF_CDP_PORT -ErrorAction SilentlyContinue }
  if ($null -ne $previousProofUserData) { $env:GOLC_DIALOG_PROOF_USER_DATA_FOLDER = $previousProofUserData } else { Remove-Item Env:GOLC_DIALOG_PROOF_USER_DATA_FOLDER -ErrorAction SilentlyContinue }
  if ($null -ne $previousEndpoint) { $env:GOLC_WEBVIEW2_CDP_ENDPOINT = $previousEndpoint } else { Remove-Item Env:GOLC_WEBVIEW2_CDP_ENDPOINT -ErrorAction SilentlyContinue }
  if ($null -ne $previousPath) { $env:Path = $previousPath }
  if ($null -ne $userDataDir -and (Test-Path -LiteralPath $userDataDir -PathType Container)) {
    for ($attempt = 0; $attempt -lt 20; $attempt++) {
      try {
        Remove-Item -LiteralPath $userDataDir -Recurse -Force -ErrorAction Stop
        break
      } catch {
        if ($attempt -eq 19) {
          Write-Warning "GOLC_DIALOG_PROOF_CLEANUP_FAILED: $($_.Exception.Message)"
        } else {
          Start-Sleep -Milliseconds 250
        }
      }
    }
  }
  if ($null -ne $overlayWorkspace -and (Test-Path -LiteralPath $overlayWorkspace -PathType Container)) { Remove-Item -LiteralPath $overlayWorkspace -Recurse -Force }
  Write-Evidence
}
