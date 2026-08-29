[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string]$Version,

    [Parameter(Mandatory = $true)]
    [ValidateSet("amd64")]
    [string]$Architecture,

    [switch]$DefineOnly
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$script:SupportedWindowsGoArtifacts = @{
    "1.26.6/amd64" = @{
        Uri = "https://github.com/actions/go-versions/releases/download/1.26.6-31764261251/go-1.26.6-win32-x64.zip"
        SHA256 = "45b92f9450d241708050462b576fc8a79dd88ee373f8b54104c6b5737ceaf5f7"
    }
}

function Select-GoExecutablePath {
    param(
        [Parameter(Mandatory = $true)]
        [AllowEmptyCollection()]
        [object[]]$Candidates
    )

    foreach ($candidate in $Candidates) {
        if ($null -eq $candidate) {
            continue
        }
        $sourceProperty = $candidate.PSObject.Properties["Source"]
        if ($null -eq $sourceProperty) {
            continue
        }
        $source = [string]$sourceProperty.Value
        if (-not [string]::IsNullOrWhiteSpace($source)) {
            return [string]$source
        }
    }

    throw "no usable Go application command with a non-empty Source was found on PATH"
}

function Resolve-SelectedGoExecutable {
    try {
        $commands = @(
            Get-Command go `
                -CommandType Application `
                -All `
                -ErrorAction Stop
        )
    } catch {
        throw "could not resolve a Go application command from PATH: $($_.Exception.Message)"
    }

    return [string](Select-GoExecutablePath -Candidates $commands)
}

function Invoke-GoCommand {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Executable,

        [Parameter(Mandatory = $true)]
        [string[]]$Arguments
    )

    $output = @(& $Executable @Arguments 2>&1)
    $exitCode = $LASTEXITCODE
    $text = ($output | ForEach-Object { $_.ToString() }) -join [Environment]::NewLine
    if ($exitCode -ne 0) {
        throw "$Executable $($Arguments -join ' ') failed with exit code ${exitCode}: $text"
    }
    return $text.Trim()
}

function Get-SupportedGoToolchainArtifact {
    param(
        [Parameter(Mandatory = $true)]
        [string]$RequestedVersion,

        [Parameter(Mandatory = $true)]
        [string]$RequestedArchitecture
    )

    $key = "$RequestedVersion/$RequestedArchitecture"
    if (-not $script:SupportedWindowsGoArtifacts.ContainsKey($key)) {
        throw "no verified Windows Go archive is configured for $key"
    }
    return $script:SupportedWindowsGoArtifacts[$key]
}

function Test-GoToolchainLayout {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Root,

        [Parameter(Mandatory = $true)]
        [string]$RequestedArchitecture
    )

    $toolDirectory = "windows_$RequestedArchitecture"
    $required = @(
        @{ Relative = "bin\go.exe"; Type = "Leaf" },
        @{ Relative = "pkg\tool\$toolDirectory\compile.exe"; Type = "Leaf" },
        @{ Relative = "src\internal\goarch"; Type = "Container" },
        @{ Relative = "src\runtime"; Type = "Container" }
    )
    $missing = @()
    foreach ($entry in $required) {
        $candidate = Join-Path $Root $entry.Relative
        if (-not (Test-Path -LiteralPath $candidate -PathType $entry.Type)) {
            $missing += $candidate
        }
    }

    return [pscustomobject]@{
        Root = $Root
        Complete = $missing.Count -eq 0
        Missing = $missing
    }
}

function ConvertTo-PhysicalTargetPath {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Target,

        [Parameter(Mandatory = $true)]
        [string]$Parent
    )

    if ($Target.StartsWith("\??\", [StringComparison]::Ordinal)) {
        $Target = $Target.Substring(4)
    } elseif ($Target.StartsWith("\\?\", [StringComparison]::Ordinal)) {
        $Target = $Target.Substring(4)
    }
    if (-not [IO.Path]::IsPathRooted($Target)) {
        $Target = Join-Path $Parent $Target
    }
    return [IO.Path]::GetFullPath($Target)
}

function Get-GoToolchainCandidateRoots {
    param(
        [Parameter(Mandatory = $true)]
        [string]$ReportedRoot
    )

    $reported = [IO.Path]::GetFullPath($ReportedRoot)
    $physicalTargets = @()
    try {
        $item = Get-Item -LiteralPath $reported -Force
        Write-Host "GOROOT item: FullName=$($item.FullName) LinkType=$($item.LinkType) Target=$($item.Target)"
        if ($null -ne $item.PSObject.Properties["Target"]) {
            foreach ($target in @($item.Target)) {
                if (-not [string]::IsNullOrWhiteSpace([string]$target)) {
                    $physicalTargets += ConvertTo-PhysicalTargetPath `
                        -Target ([string]$target) `
                        -Parent $item.Parent.FullName
                }
            }
        }
    } catch {
        Write-Warning "could not inspect reported GOROOT item ${reported}: $($_.Exception.Message)"
    }

    $seen = @{}
    $result = @()
    foreach ($candidate in @($physicalTargets) + @($reported)) {
        $key = $candidate.ToLowerInvariant()
        if (-not $seen.ContainsKey($key)) {
            $seen[$key] = $true
            $result += $candidate
        }
    }
    return $result
}

function Invoke-GoEvalSymlinksProbe {
    param(
        [Parameter(Mandatory = $true)]
        [string]$GoExecutable
    )

    $probeRoot = if ([string]::IsNullOrWhiteSpace($env:RUNNER_TEMP)) {
        [IO.Path]::GetTempPath()
    } else {
        $env:RUNNER_TEMP
    }
    $probePath = Join-Path $probeRoot "goframe-go-toolchain-probe-$PID.go"
    $source = @'
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

func main() {
	path := filepath.Join(runtime.GOROOT(), "src", "internal", "goarch")
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if !info.IsDir() {
		fmt.Fprintf(os.Stderr, "%s is not a directory\n", resolved)
		os.Exit(1)
	}
	fmt.Print(resolved)
}
'@

    try {
        Set-Content -LiteralPath $probePath -Value $source -Encoding utf8NoBOM
        return Invoke-GoCommand -Executable $GoExecutable -Arguments @("run", $probePath)
    } finally {
        Remove-Item -LiteralPath $probePath -Force -ErrorAction SilentlyContinue
    }
}

function Test-GoToolchainRoot {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Root,

        [Parameter(Mandatory = $true)]
        [string]$RequestedVersion,

        [Parameter(Mandatory = $true)]
        [string]$RequestedArchitecture
    )

    $layout = Test-GoToolchainLayout -Root $Root -RequestedArchitecture $RequestedArchitecture
    if (-not $layout.Complete) {
        throw "Go toolchain root $Root is incomplete; missing: $($layout.Missing -join ', ')"
    }

    $goExecutable = Join-Path $Root "bin\go.exe"
    $savedGoRoot = $env:GOROOT
    $savedGoToolchain = $env:GOTOOLCHAIN
    $savedPath = $env:PATH
    try {
        $env:GOROOT = $Root
        $env:GOTOOLCHAIN = "local"
        $env:PATH = "$(Join-Path $Root 'bin');$savedPath"

        $versionOutput = Invoke-GoCommand -Executable $goExecutable -Arguments @("version")
        $expectedVersion = "go version go$RequestedVersion windows/$RequestedArchitecture"
        if ($versionOutput -ne $expectedVersion) {
            throw "Go version mismatch at ${Root}: got '$versionOutput', want '$expectedVersion'"
        }

        $goRootOutput = Invoke-GoCommand -Executable $goExecutable -Arguments @("env", "GOROOT")
        if ([IO.Path]::GetFullPath($goRootOutput).TrimEnd("\") -ine $Root.TrimEnd("\")) {
            throw "Go reported GOROOT '$goRootOutput', want '$Root'"
        }

        $goToolDir = Invoke-GoCommand -Executable $goExecutable -Arguments @("env", "GOTOOLDIR")
        if (-not (Test-Path -LiteralPath $goToolDir -PathType Container)) {
            throw "Go reported missing GOTOOLDIR '$goToolDir'"
        }
        $goHostOS = Invoke-GoCommand -Executable $goExecutable -Arguments @("env", "GOHOSTOS")
        $goHostArch = Invoke-GoCommand -Executable $goExecutable -Arguments @("env", "GOHOSTARCH")
        if ($goHostOS -ne "windows" -or $goHostArch -ne $RequestedArchitecture) {
            throw "Go host mismatch: got $goHostOS/$goHostArch, want windows/$RequestedArchitecture"
        }

        $goarchDirectory = Invoke-GoCommand `
            -Executable $goExecutable `
            -Arguments @("list", "-f", "{{.Dir}}", "internal/goarch")
        if (-not (Test-Path -LiteralPath $goarchDirectory -PathType Container)) {
            throw "go list returned missing internal/goarch directory '$goarchDirectory'"
        }

        $resolvedGoarch = Invoke-GoEvalSymlinksProbe -GoExecutable $goExecutable
        Write-Host "Verified Go $RequestedVersion at $Root"
        Write-Host "GOTOOLDIR: $goToolDir"
        Write-Host "internal/goarch: $goarchDirectory"
        Write-Host "EvalSymlinks internal/goarch: $resolvedGoarch"
    } finally {
        if ($null -eq $savedGoRoot) {
            Remove-Item Env:GOROOT -ErrorAction SilentlyContinue
        } else {
            $env:GOROOT = $savedGoRoot
        }
        if ($null -eq $savedGoToolchain) {
            Remove-Item Env:GOTOOLCHAIN -ErrorAction SilentlyContinue
        } else {
            $env:GOTOOLCHAIN = $savedGoToolchain
        }
        $env:PATH = $savedPath
    }
}

function Install-VerifiedGoToolchainArchive {
    param(
        [Parameter(Mandatory = $true)]
        [string]$ArchivePath,

        [Parameter(Mandatory = $true)]
        [string]$ExpectedSHA256,

        [Parameter(Mandatory = $true)]
        [string]$DestinationRoot,

        [Parameter(Mandatory = $true)]
        [string]$RequestedArchitecture
    )

    $actualSHA256 = (Get-FileHash -LiteralPath $ArchivePath -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($actualSHA256 -ne $ExpectedSHA256.ToLowerInvariant()) {
        throw "Go archive checksum mismatch: got $actualSHA256, want $($ExpectedSHA256.ToLowerInvariant())"
    }

    if (Test-Path -LiteralPath $DestinationRoot) {
        throw "Go fallback destination already exists: $DestinationRoot"
    }
    Expand-Archive -LiteralPath $ArchivePath -DestinationPath $DestinationRoot

    $layout = Test-GoToolchainLayout `
        -Root $DestinationRoot `
        -RequestedArchitecture $RequestedArchitecture
    if (-not $layout.Complete) {
        Remove-Item -LiteralPath $DestinationRoot -Recurse -Force -ErrorAction SilentlyContinue
        throw "verified Go archive produced an incomplete installation; missing: $($layout.Missing -join ', ')"
    }
    return $DestinationRoot
}

function Install-GoToolchainFallback {
    param(
        [Parameter(Mandatory = $true)]
        [string]$RequestedVersion,

        [Parameter(Mandatory = $true)]
        [string]$RequestedArchitecture
    )

    if ([string]::IsNullOrWhiteSpace($env:RUNNER_TEMP)) {
        throw "RUNNER_TEMP is required for verified Go toolchain recovery"
    }
    $artifact = Get-SupportedGoToolchainArtifact `
        -RequestedVersion $RequestedVersion `
        -RequestedArchitecture $RequestedArchitecture
    $workRoot = Join-Path $env:RUNNER_TEMP "goframe-go-$RequestedVersion-$RequestedArchitecture-$PID"
    $archivePath = Join-Path $workRoot "go-$RequestedVersion-win32-x64.zip"
    $installationRoot = Join-Path $workRoot "installation"
    New-Item -ItemType Directory -Path $workRoot | Out-Null

    Write-Warning "setup-go installation is unusable; downloading the pinned Go $RequestedVersion archive"
    Invoke-WebRequest -Uri $artifact.Uri -OutFile $archivePath
    return Install-VerifiedGoToolchainArchive `
        -ArchivePath $archivePath `
        -ExpectedSHA256 $artifact.SHA256 `
        -DestinationRoot $installationRoot `
        -RequestedArchitecture $RequestedArchitecture
}

function Export-GoToolchainEnvironment {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Root
    )

    if ([string]::IsNullOrWhiteSpace($env:GITHUB_ENV) -or
        [string]::IsNullOrWhiteSpace($env:GITHUB_PATH)) {
        throw "GITHUB_ENV and GITHUB_PATH are required to publish the verified Go toolchain"
    }

    $bin = Join-Path $Root "bin"
    $newline = [Environment]::NewLine
    [IO.File]::AppendAllText($env:GITHUB_ENV, "GOROOT=$Root${newline}GOTOOLCHAIN=local${newline}")
    [IO.File]::AppendAllText($env:GITHUB_PATH, "$bin$newline")
    $env:GOROOT = $Root
    $env:GOTOOLCHAIN = "local"
    $env:PATH = "$bin;$env:PATH"
}

function Write-GoToolchainDiagnostics {
    param(
        [Parameter(Mandatory = $true)]
        [string]$GoExecutable,

        [Parameter(Mandatory = $true)]
        [string]$ReportedRoot,

        [Parameter(Mandatory = $true)]
        [string]$RequestedArchitecture
    )

    Write-Host "go version: $(Invoke-GoCommand -Executable $GoExecutable -Arguments @('version'))"
    foreach ($name in @("GOROOT", "GOTOOLDIR", "GOHOSTOS", "GOHOSTARCH")) {
        Write-Host "go env ${name}: $(Invoke-GoCommand -Executable $GoExecutable -Arguments @('env', $name))"
    }
    foreach ($candidate in @(Get-GoToolchainCandidateRoots -ReportedRoot $ReportedRoot)) {
        $layout = Test-GoToolchainLayout -Root $candidate -RequestedArchitecture $RequestedArchitecture
        Write-Host "candidate root: $candidate"
        Write-Host "  complete layout: $($layout.Complete)"
        foreach ($missing in $layout.Missing) {
            Write-Host "  missing: $missing"
        }
        $internal = Join-Path $candidate "src\internal"
        if (Test-Path -LiteralPath $internal -PathType Container) {
            $names = @(Get-ChildItem -LiteralPath $internal -ErrorAction SilentlyContinue |
                Select-Object -ExpandProperty Name)
            Write-Host "  src/internal entries: $($names -join ', ')"
        }
    }

    try {
        $directory = Invoke-GoCommand `
            -Executable $GoExecutable `
            -Arguments @("list", "-f", "{{.Dir}}", "internal/goarch")
        Write-Host "initial go list internal/goarch: $directory"
    } catch {
        Write-Warning "initial go list internal/goarch failed: $($_.Exception.Message)"
    }
}

function Ensure-GoToolchain {
    param(
        [Parameter(Mandatory = $true)]
        [string]$RequestedVersion,

        [Parameter(Mandatory = $true)]
        [string]$RequestedArchitecture
    )

    [void](Get-SupportedGoToolchainArtifact `
        -RequestedVersion $RequestedVersion `
        -RequestedArchitecture $RequestedArchitecture)

    $initialGo = Resolve-SelectedGoExecutable
    $reportedRoot = Invoke-GoCommand -Executable $initialGo -Arguments @("env", "GOROOT")
    Write-GoToolchainDiagnostics `
        -GoExecutable $initialGo `
        -ReportedRoot $reportedRoot `
        -RequestedArchitecture $RequestedArchitecture

    $selectedRoot = $null
    foreach ($candidate in @(Get-GoToolchainCandidateRoots -ReportedRoot $reportedRoot)) {
        try {
            Test-GoToolchainRoot `
                -Root $candidate `
                -RequestedVersion $RequestedVersion `
                -RequestedArchitecture $RequestedArchitecture
            $selectedRoot = $candidate
            break
        } catch {
            Write-Warning "rejecting Go toolchain candidate ${candidate}: $($_.Exception.Message)"
        }
    }

    if ($null -eq $selectedRoot) {
        $selectedRoot = Install-GoToolchainFallback `
            -RequestedVersion $RequestedVersion `
            -RequestedArchitecture $RequestedArchitecture
        Test-GoToolchainRoot `
            -Root $selectedRoot `
            -RequestedVersion $RequestedVersion `
            -RequestedArchitecture $RequestedArchitecture
    }

    Export-GoToolchainEnvironment -Root $selectedRoot
    Test-GoToolchainRoot `
        -Root $selectedRoot `
        -RequestedVersion $RequestedVersion `
        -RequestedArchitecture $RequestedArchitecture
    Write-Host "Published verified Go toolchain environment from $selectedRoot"
}

if (-not $DefineOnly) {
    Ensure-GoToolchain -RequestedVersion $Version -RequestedArchitecture $Architecture
}
