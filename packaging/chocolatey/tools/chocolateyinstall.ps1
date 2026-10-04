$ErrorActionPreference = 'Stop'
$version = '1.0.2'
$base = "https://github.com/AymanZahran/ai9s/releases/download/v$version"
if ($env:PROCESSOR_ARCHITECTURE -eq 'ARM64') {
  $url = "$base/ai9s_Windows_arm64.zip"
  $checksum = 'e7c027f002b9b5998660e5194c06ad86282ec442a98347dccfb0578e053c7721'
} else {
  $url = "$base/ai9s_Windows_amd64.zip"
  $checksum = 'c14bb488dd0e925c345b095196b1d54ebbf25043351cb34f3b5bbcb53bb99202'
}
$toolsDir = Split-Path -Parent $MyInvocation.MyCommand.Definition
Install-ChocolateyZipPackage `
  -PackageName 'ai9s' `
  -Url64bit $url `
  -Checksum64 $checksum `
  -ChecksumType64 'sha256' `
  -UnzipLocation $toolsDir
