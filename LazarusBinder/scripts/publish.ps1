param(
    [string]$Owner = '',
    [switch]$SkipRelease
)

$ErrorActionPreference = 'Stop'
$repoRoot = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))

function Invoke-Checked {
    param([string]$Program, [string[]]$Arguments)
    & $Program @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "$Program failed with exit code $LASTEXITCODE. Resolve the error and rerun this script."
    }
}

# Expected lookup misses must work with Windows PowerShell's native stderr
# handling as well as PowerShell 7. Return output and status separately.
function Invoke-Query {
    param([string]$Program, [string[]]$Arguments)
    $previousPreference = $ErrorActionPreference
    try {
        $ErrorActionPreference = 'Continue'
        $output = & $Program @Arguments 2>$null
        return [pscustomobject]@{
            ExitCode = $LASTEXITCODE
            Output = (@($output) -join [Environment]::NewLine)
        }
    } finally {
        $ErrorActionPreference = $previousPreference
    }
}

foreach ($program in @('git', 'gh')) {
    if (-not (Get-Command $program -ErrorAction SilentlyContinue)) {
        throw "Install $program, reopen the terminal, then rerun this script. See docs/GITHUB.md."
    }
}

Push-Location $repoRoot
try {
    $settings = Get-Content '.github/repository.json' -Raw | ConvertFrom-Json
    $version = (Get-Content 'VERSION' -Raw).Trim()
    if ($version -notmatch '^[0-9]+\.[0-9]+\.[0-9]+(?:-[A-Za-z0-9]+(?:[.-][A-Za-z0-9]+)*)?$') {
        throw 'VERSION must contain a semantic version such as 1.0.0.'
    }
    $tag = "v$version"
    if (-not (Test-Path "docs/releases/$tag.md")) {
        throw "Add release notes at docs/releases/$tag.md before publishing."
    }

    Invoke-Checked -Program 'gh' -Arguments @('auth', 'status', '--hostname', 'github.com')
    $accountJson = & gh api user --hostname github.com
    if ($LASTEXITCODE -ne 0) { throw 'Could not read the signed-in GitHub account.' }
    $githubAccount = $accountJson | ConvertFrom-Json
    if ([string]::IsNullOrWhiteSpace($Owner)) { $Owner = $settings.owner }
    if ($Owner -ne $githubAccount.login) {
        throw "Sign in as $Owner with gh auth switch, or use -Owner $($githubAccount.login) after updating the README links."
    }
    if ($settings.visibility -ne 'public') {
        throw 'This script is configured to create the public repository described in .github/repository.json.'
    }
    $repository = "$Owner/$($settings.name)"
    $repoUrl = "https://github.com/$repository"
    Write-Host "Publishing public repository: $repoUrl"

    # Initialize only this project folder, even when a parent folder uses Git.
    if (-not (Test-Path '.git')) {
        Invoke-Checked -Program 'git' -Arguments @('init', '--initial-branch=main')
    }
    $gitRoot = Invoke-Query -Program 'git' -Arguments @('rev-parse', '--show-toplevel')
    if ($gitRoot.ExitCode -ne 0 -or [System.IO.Path]::GetFullPath($gitRoot.Output.Trim()) -ne $repoRoot) {
        throw 'The Git checkout root must be this LazarusBinder folder.'
    }
    $branch = Invoke-Query -Program 'git' -Arguments @('branch', '--show-current')
    if ($branch.ExitCode -ne 0 -or $branch.Output.Trim() -ne 'main') {
        throw 'Switch this project to the main branch before publishing.'
    }

    $originProbe = Invoke-Query -Program 'git' -Arguments @('remote', 'get-url', 'origin')
    $origin = $originProbe.Output
    if ($originProbe.ExitCode -eq 0) {
        $validOrigins = @("$repoUrl.git", $repoUrl, "git@github.com:$repository.git", "ssh://git@github.com/$repository.git")
        if ($origin.Trim() -notin $validOrigins) {
            throw "The origin remote points to another repository: $origin"
        }
    }

    $authorName = Invoke-Query -Program 'git' -Arguments @('config', '--get', 'user.name')
    if ($authorName.ExitCode -ne 0 -or [string]::IsNullOrWhiteSpace($authorName.Output)) {
        Invoke-Checked -Program 'git' -Arguments @('config', 'user.name', $githubAccount.login)
    }
    $authorEmail = Invoke-Query -Program 'git' -Arguments @('config', '--get', 'user.email')
    if ($authorEmail.ExitCode -ne 0 -or [string]::IsNullOrWhiteSpace($authorEmail.Output)) {
        $noReply = "$($githubAccount.id)+$($githubAccount.login)@users.noreply.github.com"
        Invoke-Checked -Program 'git' -Arguments @('config', 'user.email', $noReply)
    }

    Invoke-Checked -Program 'git' -Arguments @('add', '--all')
    $staged = Invoke-Query -Program 'git' -Arguments @('diff', '--cached', '--quiet')
    if ($staged.ExitCode -eq 1) {
        Invoke-Checked -Program 'git' -Arguments @('commit', '-m', 'Prepare LazarusBinder for public release')
    } elseif ($staged.ExitCode -ne 0) {
        throw 'Could not check staged changes.'
    }

    Invoke-Checked -Program 'gh' -Arguments @('auth', 'setup-git', '--hostname', 'github.com')
    $existing = Invoke-Query -Program 'gh' -Arguments @('repo', 'view', $repository, '--json', 'nameWithOwner')
    $repoExists = $existing.ExitCode -eq 0
    if (-not $repoExists) {
        if (-not [string]::IsNullOrWhiteSpace($origin)) {
            throw 'An origin remote exists, but GitHub could not resolve the repository. Check account access before retrying.'
        }
        Invoke-Checked -Program 'gh' -Arguments @(
            'repo', 'create', $repository, '--public', '--source', '.',
            '--remote', 'origin', '--push', '--description', $settings.description,
            '--disable-wiki'
        )
    } else {
        if ([string]::IsNullOrWhiteSpace($origin)) {
            Invoke-Checked -Program 'git' -Arguments @('remote', 'add', 'origin', "$repoUrl.git")
        }
        # A conflicting remote history is rejected by Git; no force push is used.
        Invoke-Checked -Program 'git' -Arguments @('push', '--set-upstream', 'origin', 'main')
    }

    $editArguments = @(
        'repo', 'edit', $repository, '--description', $settings.description,
        '--default-branch', 'main', '--enable-issues', '--enable-wiki=false'
    )
    foreach ($topic in $settings.topics) {
        $editArguments += @('--add-topic', $topic)
    }
    Invoke-Checked -Program 'gh' -Arguments $editArguments

    if (-not $SkipRelease) {
        $currentCommit = Invoke-Query -Program 'git' -Arguments @('rev-parse', 'HEAD')
        if ($currentCommit.ExitCode -ne 0) { throw 'Could not read the current commit.' }
        $tagCommit = Invoke-Query -Program 'git' -Arguments @('rev-parse', '--verify', '--quiet', "$tag^{}")
        if ($tagCommit.ExitCode -eq 0) {
            if ($tagCommit.Output.Trim() -ne $currentCommit.Output.Trim()) {
                throw "$tag points to a different commit. Use a new version rather than moving a released tag."
            }
        } else {
            Invoke-Checked -Program 'git' -Arguments @('tag', '-a', $tag, '-m', "LazarusBinder $tag")
        }
        Invoke-Checked -Program 'git' -Arguments @('push', 'origin', $tag)
        Write-Host "Pushed $tag. GitHub Actions will publish the ZIP and checksums after both test jobs pass."
    }

    Write-Host "Repository: $repoUrl"
    Write-Host "Build status: $repoUrl/actions"
    Write-Host "Releases: $repoUrl/releases"
} finally {
    Pop-Location
}

