$ErrorActionPreference = 'Stop'
$version = '1.0.9'
$base = "https://github.com/AymanZahran/ai9s/releases/download/v$version"
if ($env:PROCESSOR_ARCHITECTURE -eq 'ARM64') {
  $url = "$base/ai9s_Windows_arm64.zip"
  $checksum = '435f491d786e37a26aff83f791e426b1be0f1f968dae36e9fa678503e3cdff3e'
} else {
  $url = "$base/ai9s_Windows_amd64.zip"
  $checksum = 'a6d9893347b8d1e75429b0441981ac116a69b89ee167abf3bb3e6788d211410c'
}
$toolsDir = Split-Path -Parent $MyInvocation.MyCommand.Definition
Install-ChocolateyZipPackage `
  -PackageName 'ai9s' `
  -Url64bit $url `
  -Checksum64 $checksum `
  -ChecksumType64 'sha256' `
  -UnzipLocation $toolsDir
