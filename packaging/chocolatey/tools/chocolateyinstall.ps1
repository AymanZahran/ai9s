$ErrorActionPreference = 'Stop'
$version = '1.0.11'
$base = "https://github.com/AymanZahran/ai9s/releases/download/v$version"
if ($env:PROCESSOR_ARCHITECTURE -eq 'ARM64') {
  $url = "$base/ai9s_Windows_arm64.zip"
  $checksum = 'e3032734c558fc457126c906887d24dd24206af38cb9c9b00b7afb721acd2a3c'
} else {
  $url = "$base/ai9s_Windows_amd64.zip"
  $checksum = 'd0c42b1fcf64b1220cc3449e9289c6e6c556d933dd2f65919945f74e06bf6695'
}
$toolsDir = Split-Path -Parent $MyInvocation.MyCommand.Definition
Install-ChocolateyZipPackage `
  -PackageName 'ai9s' `
  -Url64bit $url `
  -Checksum64 $checksum `
  -ChecksumType64 'sha256' `
  -UnzipLocation $toolsDir
