# USCN GPU Agent 1.0.1. Run in an administrator PowerShell terminal.
# Source: https://github.com/beijingcao/uscn-gpu-agent
param([switch]$GitHub, [switch]$DownloadOnly)
$ErrorActionPreference = 'Stop'
if (-not [Environment]::Is64BitOperatingSystem -or $env:PROCESSOR_ARCHITECTURE -ne 'AMD64') { throw 'Windows x86_64 is required.' }
if (-not $DownloadOnly) {
    $identity = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = New-Object Security.Principal.WindowsPrincipal($identity)
    if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) { throw '请在管理员PowerShell中运行安装命令。' }
}
$base = 'https://uscn.si/downloads/gpu-agent/v1.0.1'
if ($GitHub) { $base = 'https://github.com/beijingcao/uscn-gpu-agent/releases/download/v1.0.1' }
$work = Join-Path ([IO.Path]::GetTempPath()) ('uscn-gpu-install-' + [Guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $work | Out-Null
$keepPackage = $false
try {
    $archive = Join-Path $work 'uscn-gpu-agent-windows-amd64.zip'
    Invoke-WebRequest -UseBasicParsing -Uri ($base + '/uscn-gpu-agent-windows-amd64.zip') -OutFile $archive
    if ((Get-FileHash -Algorithm SHA256 -LiteralPath $archive).Hash.ToLowerInvariant() -ne 'b4fec1e38b764cc44ce530e41a672a8a9960563d80fd465e48360ac6b3b53969') { throw 'Package SHA256 mismatch; installation stopped.' }
    Expand-Archive -LiteralPath $archive -DestinationPath $work
    $package = Join-Path $work 'uscn-gpu-agent-windows-amd64'
    $setup = Join-Path $package 'install.ps1'
    if (-not (Test-Path -LiteralPath $setup) -or -not (Test-Path -LiteralPath (Join-Path $package 'SHA256SUMS'))) { throw 'Invalid package layout.' }
    if ($DownloadOnly) {
        Write-Host ('Verified USCN GPU Agent 1.0.1: ' + $package)
        $keepPackage = $true
    } else {
        Write-Host '请准备GPU上架页的一次性配对码。'
        # This process-only option does not change the machine execution policy.
        & powershell.exe -NoProfile -ExecutionPolicy Bypass -File $setup
        if ($LASTEXITCODE -ne 0) { throw 'USCN GPU Agent installation failed.' }
    }
} finally {
    if (-not $keepPackage) { Remove-Item -LiteralPath $work -Recurse -Force }
}
