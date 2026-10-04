$ErrorActionPreference = 'Stop'
$version = '1.0.5'
$base = "https://github.com/AymanZahran/ai9s/releases/download/v$version"
if ($env:PROCESSOR_ARCHITECTURE -eq 'ARM64') {
  $url = "$base/ai9s_Windows_arm64.zip"
  $checksum = 'dc2694f1f770a4727f0888727d7d7527291e339159c982edef0e01975f671e85'
} else {
  $url = "$base/ai9s_Windows_amd64.zip"
  $checksum = '44831208614d2e13191f9a5b2fbe1505df7146fd2106fe6f176acf1fedbc7682'
}
$toolsDir = Split-Path -Parent $MyInvocation.MyCommand.Definition
Install-ChocolateyZipPackage `
  -PackageName 'ai9s' `
  -Url64bit $url `
  -Checksum64 $checksum `
  -ChecksumType64 'sha256' `
  -UnzipLocation $toolsDir
