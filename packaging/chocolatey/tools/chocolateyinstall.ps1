$ErrorActionPreference = 'Stop'
$version = '1.0.3'
$base = "https://github.com/AymanZahran/ai9s/releases/download/v$version"
if ($env:PROCESSOR_ARCHITECTURE -eq 'ARM64') {
  $url = "$base/ai9s_Windows_arm64.zip"
  $checksum = 'b2316397b3641601a6e6b25d57d3449e069f119da0638a54466c85c5dbb47f99'
} else {
  $url = "$base/ai9s_Windows_amd64.zip"
  $checksum = '15e44a3bf07d999e1b3fb304286785d13f7acd064b89bd17581c718f5dda318c'
}
$toolsDir = Split-Path -Parent $MyInvocation.MyCommand.Definition
Install-ChocolateyZipPackage `
  -PackageName 'ai9s' `
  -Url64bit $url `
  -Checksum64 $checksum `
  -ChecksumType64 'sha256' `
  -UnzipLocation $toolsDir
