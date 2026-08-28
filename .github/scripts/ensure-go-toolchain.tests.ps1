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

$multipleCommands = @(
    [pscustomobject]@{ Source = "first-go.exe" },
    [pscustomobject]@{ Source = "second-go.exe" }
)
$selectedMultiple = Select-GoExecutablePath -Candidates $multipleCommands
Assert-Condition `
    -Condition ($selectedMultiple.GetType().FullName -eq "System.String") `
    -Message "multiple Go application commands did not resolve to one scalar executable path"
Assert-Condition `
    -Condition ($selectedMultiple -ceq "first-go.exe") `
    -Message "multiple Go application commands did not preserve command precedence"

$selectedSingle = Select-GoExecutablePath `
    -Candidates @([pscustomobject]@{ Source = "only-go.exe" })
Assert-Condition `
    -Condition ($selectedSingle -ceq "only-go.exe") `
    -Message "single Go application command was not returned unchanged"

$selectedAfterEmpty = Select-GoExecutablePath -Candidates @(
    [pscustomobject]@{ Source = "" },
    [pscustomobject]@{ Source = "valid-go.exe" }
)
Assert-Condition `
    -Condition ($selectedAfterEmpty -ceq "valid-go.exe") `
    -Message "empty Go application command source was not skipped deterministically"

$noCandidateFailed = $false
try {
    [void](Select-GoExecutablePath -Candidates @())
} catch {
    $noCandidateFailed = $_.Exception.Message -like "*no usable Go application command*"
}
Assert-Condition `
    -Condition $noCandidateFailed `
    -Message "empty Go application command inventory did not fail actionably"

$emptyCandidateFailed = $false
try {
    [void](Select-GoExecutablePath `
        -Candidates @([pscustomobject]@{ Source = "  " }))
} catch {
    $emptyCandidateFailed = $_.Exception.Message -like "*no usable Go application command*"
}
Assert-Condition `
    -Condition $emptyCandidateFailed `
    -Message "empty Go application command source did not fail actionably"

Write-Host "Portable Go executable selection controls: ok"

if (-not $IsWindows) {
    Write-Host "Windows-only Go toolchain layout controls: skipped on non-Windows host"
    return
}
if ([string]::IsNullOrWhiteSpace($env:RUNNER_TEMP)) {
    throw "RUNNER_TEMP is required for Windows Go toolchain helper controls"
}

$testRoot = Join-Path $env:RUNNER_TEMP "goframe-go-toolchain-tests-$PID"
New-Item -ItemType Directory -Path $testRoot | Out-Null
try {
    $incompleteRoot = Join-Path $testRoot "incomplete"
    New-Item -ItemType Directory -Path (Join-Path $incompleteRoot "bin") | Out-Null
    New-Item -ItemType Directory -Path (Join-Path $incompleteRoot "pkg\tool\windows_amd64") | Out-Null
    New-Item -ItemType Directory -Path (Join-Path $incompleteRoot "src\runtime") | Out-Null
    [IO.File]::WriteAllBytes(
        (Join-Path $incompleteRoot "bin\go.exe"),
        [byte[]]::new(0)
    )
    [IO.File]::WriteAllBytes(
        (Join-Path $incompleteRoot "pkg\tool\windows_amd64\compile.exe"),
        [byte[]]::new(0)
    )

    $incompleteLayout = Test-GoToolchainLayout `
        -Root $incompleteRoot `
        -RequestedArchitecture "amd64"
    Assert-Condition `
        -Condition (-not $incompleteLayout.Complete) `
        -Message "layout accepted a synthetic installation without internal/goarch"
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
