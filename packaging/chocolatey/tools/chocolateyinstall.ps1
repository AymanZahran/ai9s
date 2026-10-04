$ErrorActionPreference = 'Stop'
$version = '1.0.4'
$base = "https://github.com/AymanZahran/ai9s/releases/download/v$version"
if ($env:PROCESSOR_ARCHITECTURE -eq 'ARM64') {
  $url = "$base/ai9s_Windows_arm64.zip"
  $checksum = '192acaa89a8c206c04268a0571ed130d218dcc08df15863cd1a97c9503529826'
} else {
  $url = "$base/ai9s_Windows_amd64.zip"
  $checksum = '34da2677443723ee8d49a1d129f3062e346fe2cdf1782c32a3252167bc2ddc63'
}
$toolsDir = Split-Path -Parent $MyInvocation.MyCommand.Definition
Install-ChocolateyZipPackage `
  -PackageName 'ai9s' `
  -Url64bit $url `
  -Checksum64 $checksum `
  -ChecksumType64 'sha256' `
  -UnzipLocation $toolsDir
