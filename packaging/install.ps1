# Run explicitly as administrator on the seller's Windows server.
$ErrorActionPreference = 'Stop'
$identity = [Security.Principal.WindowsIdentity]::GetCurrent()
$principal = New-Object Security.Principal.WindowsPrincipal($identity)
if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) { throw '请以管理员身份运行安装程序。' }
$binary = Join-Path $PSScriptRoot 'uscn-gpu-agent.exe'
$manifest = Get-Content (Join-Path $PSScriptRoot 'SHA256SUMS')
foreach ($line in $manifest) {
    if ($line -notmatch '^([a-f0-9]{64})  ([a-zA-Z0-9._-]+)$') { throw 'Invalid release manifest' }
    $expected = $Matches[1]
    $file = Join-Path $PSScriptRoot $Matches[2]
    if ((Get-FileHash -Algorithm SHA256 -LiteralPath $file).Hash.ToLowerInvariant() -ne $expected) { throw 'Release checksum mismatch' }
}
$target = Join-Path $env:ProgramFiles 'USCN GPU Agent'
$state = Join-Path $env:ProgramData 'USCN\GPUAgent'
$config = Join-Path $state 'device.json'
New-Item -ItemType Directory -Force $target,$state | Out-Null
if (Get-Service USCNGPUAgent -ErrorAction SilentlyContinue) { Stop-Service USCNGPUAgent }
Copy-Item -LiteralPath $binary -Destination (Join-Path $target 'uscn-gpu-agent.exe') -Force
$exe = Join-Path $target 'uscn-gpu-agent.exe'
$serviceCommand = '"' + $exe + '" service --config "' + $config + '"'
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
