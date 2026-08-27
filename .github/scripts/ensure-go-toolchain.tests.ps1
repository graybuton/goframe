[CmdletBinding()]
param()

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

. "$PSScriptRoot\ensure-go-toolchain.ps1" `
    -Version "1.26.6" `
    -Architecture "amd64" `
    -DefineOnly

function Assert-Condition {
    param(
        [Parameter(Mandatory = $true)]
        [bool]$Condition,

        [Parameter(Mandatory = $true)]
        [string]$Message
    )

    if (-not $Condition) {
        throw $Message
    }
}

if (-not $IsWindows) {
    throw "Windows Go toolchain helper controls require Windows"
}
if ([string]::IsNullOrWhiteSpace($env:RUNNER_TEMP)) {
    throw "RUNNER_TEMP is required for Windows Go toolchain helper controls"
}

$testRoot = Join-Path $env:RUNNER_TEMP "goframe-go-toolchain-tests-$PID"
New-Item -ItemType Directory -Path $testRoot | Out-Null
try {
    $selectedGo = (Get-Command go -CommandType Application -ErrorAction Stop).Source
    $selectedToolDir = Invoke-GoCommand -Executable $selectedGo -Arguments @("env", "GOTOOLDIR")

    $incompleteRoot = Join-Path $testRoot "incomplete"
    New-Item -ItemType Directory -Path (Join-Path $incompleteRoot "bin") | Out-Null
    New-Item -ItemType Directory -Path (Join-Path $incompleteRoot "pkg\tool\windows_amd64") | Out-Null
    New-Item -ItemType Directory -Path (Join-Path $incompleteRoot "src\runtime") | Out-Null
    Copy-Item -LiteralPath $selectedGo -Destination (Join-Path $incompleteRoot "bin\go.exe")
    Copy-Item `
        -LiteralPath (Join-Path $selectedToolDir "compile.exe") `
        -Destination (Join-Path $incompleteRoot "pkg\tool\windows_amd64\compile.exe")

    $versionOutput = Invoke-GoCommand `
        -Executable (Join-Path $incompleteRoot "bin\go.exe") `
        -Arguments @("version")
    Assert-Condition `
        -Condition ($versionOutput -eq "go version go1.26.6 windows/amd64") `
        -Message "incomplete fixture did not retain the expected executable version"
    $incompleteLayout = Test-GoToolchainLayout `
        -Root $incompleteRoot `
        -RequestedArchitecture "amd64"
    Assert-Condition `
        -Condition (-not $incompleteLayout.Complete) `
        -Message "layout accepted a go.exe-only installation without internal/goarch"
    Assert-Condition `
        -Condition (($incompleteLayout.Missing -join "|") -like "*src\internal\goarch*") `
        -Message "incomplete layout did not identify the missing internal/goarch tree"

    $completeRoot = Join-Path $testRoot "complete"
    Copy-Item -LiteralPath $incompleteRoot -Destination $completeRoot -Recurse
    New-Item -ItemType Directory -Path (Join-Path $completeRoot "src\internal\goarch") | Out-Null
    $completeLayout = Test-GoToolchainLayout `
        -Root $completeRoot `
        -RequestedArchitecture "amd64"
    Assert-Condition -Condition $completeLayout.Complete -Message "complete fixture layout was rejected"

    $reportedRoot = Join-Path $testRoot "reported"
    New-Item -ItemType Junction -Path $reportedRoot -Target $completeRoot | Out-Null
    $candidates = @(Get-GoToolchainCandidateRoots -ReportedRoot $reportedRoot)
    Assert-Condition `
        -Condition ($candidates.Count -eq 2) `
        -Message "junction candidate inventory did not retain physical and reported roots"
    Assert-Condition `
        -Condition ($candidates[0].TrimEnd("\") -ieq $completeRoot.TrimEnd("\")) `
        -Message "physical junction target was not preferred over the reported path"
    Assert-Condition `
        -Condition ($candidates[1].TrimEnd("\") -ieq $reportedRoot.TrimEnd("\")) `
        -Message "reported junction path was not retained as a secondary candidate"

    $archiveSource = Join-Path $testRoot "archive-source"
    Copy-Item -LiteralPath $incompleteRoot -Destination $archiveSource -Recurse
    $archivePath = Join-Path $testRoot "incomplete.zip"
    Compress-Archive -Path (Join-Path $archiveSource "*") -DestinationPath $archivePath

    $mismatchDestination = Join-Path $testRoot "checksum-mismatch"
    $checksumFailed = $false
    try {
        Install-VerifiedGoToolchainArchive `
            -ArchivePath $archivePath `
            -ExpectedSHA256 (("0" * 64) -join "") `
            -DestinationRoot $mismatchDestination `
            -RequestedArchitecture "amd64"
    } catch {
        $checksumFailed = $_.Exception.Message -like "*checksum mismatch*"
    }
    Assert-Condition -Condition $checksumFailed -Message "checksum mismatch did not abort installation"
    Assert-Condition `
        -Condition (-not (Test-Path -LiteralPath $mismatchDestination)) `
        -Message "checksum mismatch created an extraction destination"

    $incompleteDestination = Join-Path $testRoot "fallback-incomplete"
    $archiveSHA256 = (Get-FileHash -LiteralPath $archivePath -Algorithm SHA256).Hash
    $incompleteFailed = $false
    try {
        Install-VerifiedGoToolchainArchive `
            -ArchivePath $archivePath `
            -ExpectedSHA256 $archiveSHA256 `
            -DestinationRoot $incompleteDestination `
            -RequestedArchitecture "amd64"
    } catch {
        $incompleteFailed = $_.Exception.Message -like "*incomplete installation*"
    }
    Assert-Condition `
        -Condition $incompleteFailed `
        -Message "checksum-valid but incomplete fallback installation did not abort"

    $unsupportedFailed = $false
    try {
        Get-SupportedGoToolchainArtifact `
            -RequestedVersion "1.27.0" `
            -RequestedArchitecture "amd64"
    } catch {
        $unsupportedFailed = $_.Exception.Message -like "*no verified Windows Go archive*"
    }
    Assert-Condition `
        -Condition $unsupportedFailed `
        -Message "helper accepted an unrequested Go version fallback"

    Write-Host "Windows Go toolchain integrity helper controls: ok"
} finally {
    Remove-Item -LiteralPath $testRoot -Recurse -Force -ErrorAction SilentlyContinue
}
