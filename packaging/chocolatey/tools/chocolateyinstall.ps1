$ErrorActionPreference = 'Stop'
$version = '1.0.8'
$base = "https://github.com/AymanZahran/ai9s/releases/download/v$version"
if ($env:PROCESSOR_ARCHITECTURE -eq 'ARM64') {
  $url = "$base/ai9s_Windows_arm64.zip"
  $checksum = '8f1f48cda346a9c5561e79705e4ed065652b206448ddd30465fd51d6b6ee0e51'
} else {
  $url = "$base/ai9s_Windows_amd64.zip"
  $checksum = '0824a7c7bc0559bee3653e969c02bf1a6d58027cfbe91412af059e0e552f16bc'
}
$toolsDir = Split-Path -Parent $MyInvocation.MyCommand.Definition
Install-ChocolateyZipPackage `
  -PackageName 'ai9s' `
  -Url64bit $url `
  -Checksum64 $checksum `
  -ChecksumType64 'sha256' `
  -UnzipLocation $toolsDir
