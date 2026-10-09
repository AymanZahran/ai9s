$ErrorActionPreference = 'Stop'
$version = '1.0.7'
$base = "https://github.com/AymanZahran/ai9s/releases/download/v$version"
if ($env:PROCESSOR_ARCHITECTURE -eq 'ARM64') {
  $url = "$base/ai9s_Windows_arm64.zip"
  $checksum = 'd1649de221004e07e87549c154a7be92596cb09cca2d6150ae24326781fd3ea9'
} else {
  $url = "$base/ai9s_Windows_amd64.zip"
  $checksum = 'bebf0c86048464a4349b8b5dbcd9fdd7b6367ddfb651e9566210842d97e0f5b3'
}
$toolsDir = Split-Path -Parent $MyInvocation.MyCommand.Definition
Install-ChocolateyZipPackage `
  -PackageName 'ai9s' `
  -Url64bit $url `
  -Checksum64 $checksum `
  -ChecksumType64 'sha256' `
  -UnzipLocation $toolsDir
