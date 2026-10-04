$ErrorActionPreference = 'Stop'
$version = '1.0.6'
$base = "https://github.com/AymanZahran/ai9s/releases/download/v$version"
if ($env:PROCESSOR_ARCHITECTURE -eq 'ARM64') {
  $url = "$base/ai9s_Windows_arm64.zip"
  $checksum = 'bee4dbed875be2a072b8af7be501bd8a8f57bb374593ce29cb8bbd79b8b88b7b'
} else {
  $url = "$base/ai9s_Windows_amd64.zip"
  $checksum = '91b30a021a6b9cf24d531e07a348e9bbb5417e504f1eb7179990b7d4a9564753'
}
$toolsDir = Split-Path -Parent $MyInvocation.MyCommand.Definition
Install-ChocolateyZipPackage `
  -PackageName 'ai9s' `
  -Url64bit $url `
  -Checksum64 $checksum `
  -ChecksumType64 'sha256' `
  -UnzipLocation $toolsDir
