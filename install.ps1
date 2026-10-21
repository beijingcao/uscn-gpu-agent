# USCN GPU Agent 1.0.3. Run in an administrator PowerShell terminal.
# Source: https://github.com/beijingcao/uscn-gpu-agent
param([switch]$GitHub, [switch]$DownloadOnly, [ValidateSet('Install','Uninstall')][string]$Action)
$ErrorActionPreference = 'Stop'
if ($DownloadOnly -and $Action) { throw 'DownloadOnly cannot be combined with an action.' }
if (-not $DownloadOnly -and -not $Action) {
    Write-Host 'USCN GPU Agent 1.0.3'
    Write-Host '  1) 安装 / 升级'
    Write-Host '  2) 卸载'
    Write-Host '  0) 退出'
    $choice = Read-Host '请选择 [0/1/2，默认0]'
    switch ($choice) {
        '1' { $Action = 'Install' }
        '2' { $Action = 'Uninstall' }
        { $_ -eq '0' -or $_ -eq '' } { Write-Host '已退出。'; return }
        default { throw '无效选项，未执行任何操作。' }
    }
}
if (-not [Environment]::Is64BitOperatingSystem -or $env:PROCESSOR_ARCHITECTURE -ne 'AMD64') { throw 'Windows x86_64 is required.' }
if (-not $DownloadOnly) {
    $identity = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = New-Object Security.Principal.WindowsPrincipal($identity)
    if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) { throw '请在管理员PowerShell中运行安装命令。' }
}
$base = 'https://uscn.si/downloads/gpu-agent/v1.0.3'
if ($GitHub) { $base = 'https://github.com/beijingcao/uscn-gpu-agent/releases/download/v1.0.3' }
$work = Join-Path ([IO.Path]::GetTempPath()) ('uscn-gpu-install-' + [Guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $work | Out-Null
$keepPackage = $false
try {
    $archive = Join-Path $work 'uscn-gpu-agent-windows-amd64.zip'
    Invoke-WebRequest -UseBasicParsing -Uri ($base + '/uscn-gpu-agent-windows-amd64.zip') -OutFile $archive
    if ((Get-FileHash -Algorithm SHA256 -LiteralPath $archive).Hash.ToLowerInvariant() -ne '4b21be36f37b8530b266e61d3c398e0f35d87b874dc7d5b3706c3d3168f2f2f8') { throw 'Package SHA256 mismatch; installation stopped.' }
    Expand-Archive -LiteralPath $archive -DestinationPath $work
    $package = Join-Path $work 'uscn-gpu-agent-windows-amd64'
    $setup = Join-Path $package 'install.ps1'
    if (-not (Test-Path -LiteralPath $setup) -or -not (Test-Path -LiteralPath (Join-Path $package 'SHA256SUMS'))) { throw 'Invalid package layout.' }
    if ($DownloadOnly) {
        Write-Host ('Verified USCN GPU Agent 1.0.3: ' + $package)
        $keepPackage = $true
    } else {
        if ($Action -eq 'Install') { Write-Host '请准备GPU上架页的一次性配对码。' } else { Write-Host '卸载将停止上报并移除程序；保留设备身份与待传报告，不解除平台资源或订单。' }
        # This process-only option does not change the machine execution policy.
        & powershell.exe -NoProfile -ExecutionPolicy Bypass -File $setup -Action $Action
        if ($LASTEXITCODE -ne 0) { throw 'USCN GPU Agent operation failed.' }
    }
} finally {
    if (-not $keepPackage) { Remove-Item -LiteralPath $work -Recurse -Force }
}
