param([ValidateSet('Install','Uninstall')][string]$Action)
# Run explicitly as administrator on the seller's Windows server.
$ErrorActionPreference = 'Stop'
if (-not $Action) {
    Write-Host '  1) 安装 / 升级'
    Write-Host '  2) 卸载（保留设备身份与待传报告）'
    Write-Host '  0) 退出'
    $choice = Read-Host '请选择 [0/1/2，默认0]'
    switch ($choice) {
        '1' { $Action = 'Install' }
        '2' { $Action = 'Uninstall' }
        { $_ -eq '0' -or $_ -eq '' } { return }
        default { throw '无效选项，未执行任何操作。' }
    }
}
$identity = [Security.Principal.WindowsIdentity]::GetCurrent()
$principal = New-Object Security.Principal.WindowsPrincipal($identity)
if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) { throw '请以管理员身份运行安装程序。' }
$target = Join-Path $env:ProgramFiles 'USCN GPU Agent'
$state = Join-Path $env:ProgramData 'USCN\GPUAgent'
$config = Join-Path $state 'device.json'
$exe = Join-Path $target 'uscn-gpu-agent.exe'
$serviceCommand = '"' + $exe + '" service --config "' + $config + '"'
if ($Action -eq 'Uninstall') {
    foreach ($path in @($target,$exe)) {
        if (Test-Path -LiteralPath $path) {
            if ((Get-Item -LiteralPath $path -Force).Attributes -band [IO.FileAttributes]::ReparsePoint) { throw '插件程序路径为重解析点，停止卸载。' }
        }
    }
    $service = Get-CimInstance -ClassName Win32_Service -Filter "Name='USCNGPUAgent'"
    if ($service) {
        if (-not [string]::Equals($service.PathName, $serviceCommand, [StringComparison]::OrdinalIgnoreCase)) { throw '服务文件与USCN GPU插件不一致，停止卸载。' }
        $status = Get-Service -Name USCNGPUAgent
        if ($status.Status -ne 'Stopped') {
            Stop-Service -Name USCNGPUAgent -ErrorAction Stop
            (Get-Service -Name USCNGPUAgent).WaitForStatus('Stopped', [TimeSpan]::FromSeconds(30))
        }
        & sc.exe delete USCNGPUAgent
        if ($LASTEXITCODE -ne 0) { throw 'Service deletion failed; device state retained.' }
    }
    if (Test-Path -LiteralPath $exe) { Remove-Item -LiteralPath $exe -Force }
    if ((Test-Path -LiteralPath $target) -and @(Get-ChildItem -LiteralPath $target -Force).Count -eq 0) { Remove-Item -LiteralPath $target }
    Write-Host 'USCN GPU插件已卸载或未安装。设备身份与待传报告已保留，可重新安装恢复。'
    Write-Host '设备将停止上报；平台资源、审核记录和订单义务不因本机卸载而解除。'
    return
}
$binary = Join-Path $PSScriptRoot 'uscn-gpu-agent.exe'
$manifest = Get-Content (Join-Path $PSScriptRoot 'SHA256SUMS')
foreach ($line in $manifest) {
    if ($line -notmatch '^([a-f0-9]{64})  ([a-zA-Z0-9._-]+)$') { throw 'Invalid release manifest' }
    $expected = $Matches[1]
    $file = Join-Path $PSScriptRoot $Matches[2]
    if ((Get-FileHash -Algorithm SHA256 -LiteralPath $file).Hash.ToLowerInvariant() -ne $expected) { throw 'Release checksum mismatch' }
}
New-Item -ItemType Directory -Force $target,$state | Out-Null
if (Get-Service USCNGPUAgent -ErrorAction SilentlyContinue) { Stop-Service USCNGPUAgent }
Copy-Item -LiteralPath $binary -Destination (Join-Path $target 'uscn-gpu-agent.exe') -Force
if (-not (Get-Service USCNGPUAgent -ErrorAction SilentlyContinue)) {
    & sc.exe create USCNGPUAgent binPath= $serviceCommand start= auto obj= 'NT SERVICE\USCNGPUAgent' DisplayName= 'USCN GPU Agent'
    if ($LASTEXITCODE -ne 0) { throw 'Service creation failed' }
}
& icacls.exe $state /inheritance:r /grant:r '*S-1-5-18:(OI)(CI)F' '*S-1-5-32-544:(OI)(CI)F' 'NT SERVICE\USCNGPUAgent:(OI)(CI)M' | Out-Null
if ($LASTEXITCODE -ne 0) { throw 'Private device state ACL failed' }
if (-not (Test-Path -LiteralPath $config)) {
    $code = Read-Host '请粘贴GPU上架页面的一次性配对码' -AsSecureString
    $ptr = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($code)
    try {
        $processInfo = New-Object Diagnostics.ProcessStartInfo
        $processInfo.FileName = $exe
        $processInfo.Arguments = 'enroll --config "' + $config + '"'
        $processInfo.UseShellExecute = $false
        $processInfo.RedirectStandardInput = $true
        $process = [Diagnostics.Process]::Start($processInfo)
        $process.StandardInput.WriteLine([Runtime.InteropServices.Marshal]::PtrToStringBSTR($ptr))
        $process.StandardInput.Close()
        $process.WaitForExit()
        if ($process.ExitCode -ne 0) { throw 'Device pairing failed' }
    } finally { [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($ptr) }
}
Start-Service USCNGPUAgent
Write-Host 'USCN GPU插件已启动，请回到上架页查看采集结果。'
