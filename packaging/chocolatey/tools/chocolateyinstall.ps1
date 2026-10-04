$ErrorActionPreference = 'Stop'
$version = '1.0.1'
$base = "https://github.com/AymanZahran/ai9s/releases/download/v$version"
if ($env:PROCESSOR_ARCHITECTURE -eq 'ARM64') {
  $url = "$base/ai9s_Windows_arm64.zip"
  $checksum = '429b230dafa02e0a386a27656a6d0c93f4f391a45172dd3f3e99d8e7623e908a'
} else {
  $url = "$base/ai9s_Windows_amd64.zip"
  $checksum = '222bc709c393125aa426ce8e4649dd66a13fc5071e93af21736a16762489cf67'
}
$toolsDir = Split-Path -Parent $MyInvocation.MyCommand.Definition
Install-ChocolateyZipPackage `
  -PackageName 'ai9s' `
  -Url64bit $url `
  -Checksum64 $checksum `
  -ChecksumType64 'sha256' `
  -UnzipLocation $toolsDir
