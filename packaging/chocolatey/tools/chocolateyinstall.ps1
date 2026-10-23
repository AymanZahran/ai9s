$ErrorActionPreference = 'Stop'
$version = '1.0.10'
$base = "https://github.com/AymanZahran/ai9s/releases/download/v$version"
if ($env:PROCESSOR_ARCHITECTURE -eq 'ARM64') {
  $url = "$base/ai9s_Windows_arm64.zip"
  $checksum = '87639c39cd5f29426d8a00aaa6b117bc10ba4b12ebd9b20a5188c688ebaa1dc4'
} else {
  $url = "$base/ai9s_Windows_amd64.zip"
  $checksum = 'eb9268460c74fdb64bb126a4f99debb4f568ccd855a07e29c391fc1840658368'
}
$toolsDir = Split-Path -Parent $MyInvocation.MyCommand.Definition
Install-ChocolateyZipPackage `
  -PackageName 'ai9s' `
  -Url64bit $url `
  -Checksum64 $checksum `
  -ChecksumType64 'sha256' `
  -UnzipLocation $toolsDir
