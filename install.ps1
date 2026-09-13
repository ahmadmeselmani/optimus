$ErrorActionPreference = 'Stop'
if (-not [System.Runtime.InteropServices.RuntimeInformation]::IsOSPlatform([System.Runtime.InteropServices.OSPlatform]::Windows)) {
    throw 'Use install.sh on Linux or macOS.'
}
$arch = switch ([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString()) {
    'X64' { 'amd64' }
    'Arm64' { 'arm64' }
    default { throw 'Supported Windows architectures: x64 and ARM64.' }
}
$base = 'https://github.com/ahmadmeselmani/optimus/releases/latest/download'
if ($env:OPTIMUS_VERSION) {
    if ($env:OPTIMUS_VERSION -notmatch '^v\d+\.\d+\.\d+$') { throw 'OPTIMUS_VERSION must be a release tag such as v0.1.0.' }
    $base = "https://github.com/ahmadmeselmani/optimus/releases/download/$env:OPTIMUS_VERSION"
}
$asset = "optimus_windows_$arch.zip"
$work = Join-Path ([IO.Path]::GetTempPath()) ([Guid]::NewGuid().ToString())
New-Item -ItemType Directory -Path $work | Out-Null
try {
    Write-Host "Downloading Optimus for Windows/$arch..."
    Invoke-WebRequest "$base/$asset" -OutFile "$work\$asset" -UseBasicParsing
    Invoke-WebRequest "$base/checksums.txt" -OutFile "$work\checksums.txt" -UseBasicParsing
    $entry = @(Get-Content "$work\checksums.txt" | Where-Object { $_ -match ('^[a-fA-F0-9]{64}\s+' + [regex]::Escape($asset) + '$') })
    if ($entry.Count -ne 1) { throw 'Missing or invalid archive checksum.' }
    $expected = ($entry[0] -split '\s+')[0]
    if ((Get-FileHash "$work\$asset" -Algorithm SHA256).Hash -ne $expected) { throw 'Checksum mismatch; nothing was installed.' }
    Expand-Archive "$work\$asset" -DestinationPath "$work\extracted"
    $dir = Join-Path $env:LOCALAPPDATA 'Optimus\bin'
    if ($env:OPTIMUS_INSTALL_DIR) { $dir = [IO.Path]::GetFullPath($env:OPTIMUS_INSTALL_DIR) }
    New-Item -ItemType Directory -Force -Path $dir | Out-Null
    Copy-Item "$work\extracted\optimus.exe" "$dir\optimus.exe" -Force
    $userPath = [string][Environment]::GetEnvironmentVariable('Path', 'User')
    if (($userPath -split ';') -notcontains $dir) {
        $newPath = ($userPath.TrimEnd(';') + ';' + $dir).TrimStart(';')
        [Environment]::SetEnvironmentVariable('Path', $newPath, 'User')
    }
    if (($env:Path -split ';') -notcontains $dir) { $env:Path += ";$dir" }
    Write-Host "Optimus installed at $dir\optimus.exe. Run: optimus"
    Write-Host 'Ollama must be running with a local model installed. Optimus is free; no account or payment is required.'
} finally {
    Remove-Item $work -Recurse -Force
}
