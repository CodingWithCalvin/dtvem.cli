# dtvem uninstaller for Windows
# Usage:
#   Standard:  irm https://raw.githubusercontent.com/CodingWithCalvin/dtvem.cli/main/uninstall.ps1 | iex
#   No prompt: iex "& { $(irm https://raw.githubusercontent.com/CodingWithCalvin/dtvem.cli/main/uninstall.ps1) } -Yes"
#
# Removes everything dtvem puts on a machine:
#   - the dtvem directory: the dtvem binaries, shims, every runtime version
#     installed with dtvem (and its global packages), config and cache
#   - the dtvem entries the installer and `dtvem init` added to the User and
#     System PATH
#   - what RubyInstaller registered when older dtvem versions used it to
#     install Ruby: Programs and Features entries, Start menu folders, PATH
#     entries and the .rb and .rbw file associations
# Changes to the System PATH and to machine-wide settings need administrator
# rights, so Windows asks for them (UAC) when there are any to make.
# Project files (.dtvem\runtimes.json, .tool-versions, .nvmrc, ...) are left alone.

param(
    [switch]$Yes
)

# The symbols are built from their code points so this file stays ASCII:
# Windows PowerShell 5.1 reads a script file without a BOM in the ANSI code
# page, where the UTF-8 bytes of these symbols include quote characters.

function Write-Info {
    param([string]$Message)
    Write-Host "$([char]0x2192) " -ForegroundColor Cyan -NoNewline
    Write-Host $Message
}

function Write-Success {
    param([string]$Message)
    Write-Host "$([char]0x2713) " -ForegroundColor Green -NoNewline
    Write-Host $Message
}

function Write-Error-Custom {
    param([string]$Message)
    Write-Host "$([char]0x2717) " -ForegroundColor Red -NoNewline
    Write-Host $Message
}

function Write-Warning-Custom {
    param([string]$Message)
    Write-Host "$([char]0x26A0) " -ForegroundColor Yellow -NoNewline
    Write-Host $Message
}

function ConvertTo-FullPath {
    param([string]$Path)
    return [System.IO.Path]::GetFullPath($Path).TrimEnd('\')
}

function Test-UnderDirectory {
    # Whether Path is Directory or inside it
    param([string]$Path, [string]$Directory)
    return ($Path -eq $Directory) -or $Path.StartsWith("$Directory\", [System.StringComparison]::OrdinalIgnoreCase)
}

function Test-OwnedRoot {
    # Whether a root is a directory dtvem owns outright, named dtvem or .dtvem.
    # A DTVEM_ROOT with any other name may hold other files, so only the
    # entries dtvem creates in it are removed.
    param([string]$Root)
    $name = Split-Path -Leaf $Root
    return ($name -eq 'dtvem') -or ($name -eq '.dtvem')
}

function Get-RootEntry {
    # The paths that removing a root deletes
    param([string]$Root)

    if (Test-OwnedRoot -Root $Root) {
        if (Test-Path -LiteralPath $Root) {
            $Root
        }
        return
    }

    foreach ($entry in 'shims', 'versions', 'config', 'cache', 'tmp', 'bin\dtvem.exe', 'bin\dtvem-shim.exe') {
        $path = Join-Path $Root $entry
        if (Test-Path -LiteralPath $path) {
            $path
        }
    }
}

function Get-DtvemRoot {
    # Every dtvem root on this machine: DTVEM_ROOT, XDG_DATA_HOME\dtvem, and
    # the default %USERPROFILE%\.dtvem
    $candidates = @()
    if ($env:DTVEM_ROOT) {
        $candidates += $env:DTVEM_ROOT
    }
    if ($env:XDG_DATA_HOME) {
        $candidates += Join-Path $env:XDG_DATA_HOME 'dtvem'
    }
    $candidates += Join-Path $env:USERPROFILE '.dtvem'

    $homeDir = ConvertTo-FullPath $env:USERPROFILE
    $roots = @()
    foreach ($candidate in $candidates) {
        $root = ConvertTo-FullPath $candidate
        if (-not (Test-Path -LiteralPath $root -PathType Container)) {
            continue
        }
        if (($root -eq [System.IO.Path]::GetPathRoot($root).TrimEnd('\')) -or ($root -eq $homeDir)) {
            Write-Warning-Custom "Skipping ${root}: it is a drive root or your home directory"
            continue
        }
        if ($roots -contains $root) {
            continue
        }
        if (@(Get-RootEntry -Root $root).Count -gt 0) {
            $roots += $root
        }
    }
    return $roots
}

function Get-InstalledRuntime {
    # The runtime versions installed under a root, one line per runtime
    param([string]$Root)

    $versionsDir = Join-Path $Root 'versions'
    if (-not (Test-Path -LiteralPath $versionsDir -PathType Container)) {
        return
    }
    foreach ($runtimeDir in Get-ChildItem -LiteralPath $versionsDir -Directory) {
        $versions = @(Get-ChildItem -LiteralPath $runtimeDir.FullName -Directory | ForEach-Object { $_.Name })
        if ($versions.Count -gt 0) {
            "$($runtimeDir.Name): $($versions -join ', ')"
        }
    }
}

function Test-DtvemLocation {
    # Whether a path (a PATH entry, a shortcut target, ...) belongs to dtvem:
    # it is in a dtvem root being removed, or it is gone and was in a dtvem
    # directory (a dtvem root, or the %TEMP%\dtvem-ruby-<version> directory
    # older dtvem versions ran the Ruby installer in). Anything that still
    # exists outside the roots, such as another dtvem install, is kept.
    param([string]$Path, [string[]]$Roots)

    if (-not $Path -or -not $Path.Trim()) {
        return $false
    }
    $full = [Environment]::ExpandEnvironmentVariables($Path.Trim().Trim('"')).Replace('/', '\').TrimEnd('\')

    foreach ($root in $Roots) {
        if (Test-UnderDirectory -Path $full -Directory $root) {
            return $true
        }
    }

    $dtvemDirs = @($full -split '\\' | Where-Object { ($_ -eq 'dtvem') -or ($_ -eq '.dtvem') -or ($_ -like 'dtvem-ruby-*') })
    if ($dtvemDirs.Count -eq 0) {
        return $false
    }
    try {
        return -not (Test-Path -LiteralPath $full)
    }
    catch {
        return $false
    }
}

function Get-PathValue {
    # The User or System PATH as stored in the registry, with %VAR%
    # references left unexpanded
    param([string]$Scope)

    if ($Scope -eq 'System') {
        $key = [Microsoft.Win32.Registry]::LocalMachine.OpenSubKey('SYSTEM\CurrentControlSet\Control\Session Manager\Environment')
    }
    else {
        $key = [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey('Environment')
    }
    if (-not $key) {
        return ''
    }
    try {
        return [string]$key.GetValue('Path', '', [Microsoft.Win32.RegistryValueOptions]::DoNotExpandEnvironmentNames)
    }
    finally {
        $key.Close()
    }
}

function Get-RegistryDefault {
    # The default value of a registry key, if the key exists
    param([string]$Key)
    $item = Get-Item -LiteralPath $Key -ErrorAction SilentlyContinue
    if ($item) {
        return $item.GetValue('')
    }
}

function Get-CommandProgram {
    # The program a command line such as "C:\ruby\bin\ruby.exe" "%1" %* runs
    param([string]$Command)
    if (-not $Command) {
        return ''
    }
    $line = $Command.Trim()
    if ($line.StartsWith('"')) {
        return $line.Substring(1).Split('"')[0]
    }
    return $line.Split(' ')[0]
}

function Test-DtvemShortcutFolder {
    # Whether every shortcut in a Start menu folder points into a dtvem
    # location, through its target or a path in its arguments
    param([string]$Folder, [string[]]$Roots)

    $shortcuts = @(Get-ChildItem -LiteralPath $Folder -Filter '*.lnk' -Recurse -File -ErrorAction SilentlyContinue)
    if ($shortcuts.Count -eq 0) {
        return $false
    }
    try {
        $shell = New-Object -ComObject WScript.Shell
        foreach ($file in $shortcuts) {
            $shortcut = $shell.CreateShortcut($file.FullName)
            $paths = @($shortcut.TargetPath) + @($shortcut.Arguments -split '\s+')
            if (@($paths | Where-Object { Test-DtvemLocation -Path $_ -Roots $Roots }).Count -eq 0) {
                return $false
            }
        }
    }
    catch {
        return $false
    }
    return $true
}

function Find-Leftover {
    # What dtvem left outside its root directories: PATH entries, and what
    # RubyInstaller registered when older dtvem versions used it to install
    # Ruby. Admin marks the ones that need administrator rights to remove.
    param([string[]]$Roots)

    foreach ($scope in 'User', 'System') {
        foreach ($entry in (Get-PathValue -Scope $scope) -split ';') {
            if (Test-DtvemLocation -Path $entry -Roots $Roots) {
                [pscustomobject]@{ Kind = 'Path'; Scope = $scope; Target = $entry; Admin = ($scope -eq 'System'); Label = "$scope PATH entry $($entry.Trim())" }
            }
        }
    }

    foreach ($hive in 'HKCU:', 'HKLM:') {
        $admin = $hive -eq 'HKLM:'

        $uninstallKey = "$hive\Software\Microsoft\Windows\CurrentVersion\Uninstall"
        foreach ($key in @(Get-ChildItem -LiteralPath $uninstallKey -ErrorAction SilentlyContinue)) {
            if (($key.PSChildName -like 'RubyInstaller-*') -and (Test-DtvemLocation -Path $key.GetValue('InstallLocation') -Roots $Roots)) {
                $name = $key.GetValue('DisplayName')
                if (-not $name) {
                    $name = $key.PSChildName
                }
                [pscustomobject]@{ Kind = 'RegistryKey'; Scope = ''; Target = $key.PSPath; Admin = $admin; Label = "Programs and Features entry `"$name`"" }
            }
        }

        foreach ($fileType in 'RubyFile', 'RubyWFile') {
            $typeKey = "$hive\Software\Classes\$fileType"
            $program = Get-CommandProgram -Command (Get-RegistryDefault -Key "$typeKey\shell\open\command")
            if (-not (Test-DtvemLocation -Path $program -Roots $Roots)) {
                continue
            }
            [pscustomobject]@{ Kind = 'RegistryKey'; Scope = ''; Target = $typeKey; Admin = $admin; Label = "File type $fileType, which opens $program" }
            foreach ($extension in '.rb', '.rbw') {
                $extensionKey = "$hive\Software\Classes\$extension"
                if ((Get-RegistryDefault -Key $extensionKey) -eq $fileType) {
                    [pscustomobject]@{ Kind = 'RegistryDefault'; Scope = ''; Target = $extensionKey; Admin = $admin; Label = "$extension file association" }
                }
            }
        }
    }

    $startMenus = @(
        @{ Dir = [Environment]::GetFolderPath('Programs'); Admin = $false },
        @{ Dir = [Environment]::GetFolderPath('CommonPrograms'); Admin = $true }
    )
    foreach ($menu in $startMenus) {
        if (-not $menu.Dir) {
            continue
        }
        foreach ($folder in @(Get-ChildItem -LiteralPath $menu.Dir -Directory -Filter 'Ruby *' -ErrorAction SilentlyContinue)) {
            if (Test-DtvemShortcutFolder -Folder $folder.FullName -Roots $Roots) {
                [pscustomobject]@{ Kind = 'Folder'; Scope = ''; Target = $folder.FullName; Admin = $menu.Admin; Label = "Start menu folder `"$($folder.Name)`"" }
            }
        }
    }
}

function Send-EnvironmentChange {
    # Tell running applications (Explorer, new terminals) that the
    # environment changed, as [Environment]::SetEnvironmentVariable would.
    $signature = '[DllImport("user32.dll", SetLastError = true, CharSet = CharSet.Auto)] public static extern IntPtr SendMessageTimeout(IntPtr hWnd, uint Msg, UIntPtr wParam, string lParam, uint fuFlags, uint uTimeout, out UIntPtr lpdwResult);'
    $native = Add-Type -MemberDefinition $signature -Name 'NativeMethods' -Namespace 'DtvemUninstaller' -PassThru
    $result = [UIntPtr]::Zero
    [void]$native::SendMessageTimeout([IntPtr]0xffff, 0x1A, [UIntPtr]::Zero, 'Environment', 2, 5000, [ref]$result)
}

function Clear-PathEntry {
    # Takes the given entries off the User or System PATH in the registry,
    # keeping every other entry as written. Windows only expands %VAR%
    # entries such as %SystemRoot%\system32 in a REG_EXPAND_SZ value, so PATH
    # is written back as REG_EXPAND_SZ when it was stored that way or holds a
    # %VAR% reference ([Environment]::SetEnvironmentVariable would write
    # REG_SZ).
    param([string]$Scope, [string[]]$Entry)

    if ($Scope -eq 'System') {
        $key = [Microsoft.Win32.Registry]::LocalMachine.OpenSubKey('SYSTEM\CurrentControlSet\Control\Session Manager\Environment', $true)
    }
    else {
        $key = [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey('Environment', $true)
    }
    if (-not $key) {
        throw "Could not open the $Scope environment in the registry"
    }
    try {
        if ($key.GetValueNames() -notcontains 'Path') {
            return
        }
        $value = [string]$key.GetValue('Path', '', [Microsoft.Win32.RegistryValueOptions]::DoNotExpandEnvironmentNames)
        $kind = $key.GetValueKind('Path')
        $newValue = @($value -split ';' | Where-Object { $Entry -notcontains $_ }) -join ';'
        if ($newValue -eq $value) {
            return
        }

        if (($Scope -eq 'User') -and -not $newValue.Trim(';').Trim()) {
            # The User PATH only held dtvem's entries: remove it, as it was
            # before dtvem was installed
            $key.DeleteValue('Path')
        }
        elseif (($kind -eq [Microsoft.Win32.RegistryValueKind]::ExpandString) -or ($newValue -match '%[^%;]+%')) {
            $key.SetValue('Path', $newValue, [Microsoft.Win32.RegistryValueKind]::ExpandString)
        }
        else {
            $key.SetValue('Path', $newValue, [Microsoft.Win32.RegistryValueKind]::String)
        }
    }
    finally {
        $key.Close()
    }

    try {
        Send-EnvironmentChange
    }
    catch {
        # Only running applications miss the change; new terminals still see it
        Write-Verbose "Could not announce the PATH change: $_"
    }
}

function Clear-Leftover {
    # Removes one item found by Find-Leftover
    param([object]$Leftover)

    switch ($Leftover.Kind) {
        'Path' {
            Clear-PathEntry -Scope $Leftover.Scope -Entry @($Leftover.Target)
        }
        'RegistryDefault' {
            # Target is HKCU:\<subkey> or HKLM:\<subkey>
            $hive, $subKey = $Leftover.Target -split ':\\', 2
            $base = if ($hive -eq 'HKLM') { [Microsoft.Win32.Registry]::LocalMachine } else { [Microsoft.Win32.Registry]::CurrentUser }
            $key = $base.OpenSubKey($subKey, $true)
            if ($key) {
                try {
                    $key.DeleteValue('', $false)
                }
                finally {
                    $key.Close()
                }
            }
        }
        default {
            Remove-Item -LiteralPath $Leftover.Target -Recurse -Force
        }
    }
}

function Test-Administrator {
    $identity = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = [Security.Principal.WindowsPrincipal]::new($identity)
    return $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

function Get-ElevatedCleanupScript {
    # A standalone script that removes the given leftovers
    param([object[]]$Leftover)

    $items =($Leftover | ForEach-Object {
            $kind = [System.Management.Automation.Language.CodeGeneration]::EscapeSingleQuotedStringContent($_.Kind)
            $scope = [System.Management.Automation.Language.CodeGeneration]::EscapeSingleQuotedStringContent($_.Scope)
            $target = [System.Management.Automation.Language.CodeGeneration]::EscapeSingleQuotedStringContent($_.Target)
            "    [pscustomobject]@{ Kind = '$kind'; Scope = '$scope'; Target = '$target' }"
        }) -join ",`n"

    return @"
`$ErrorActionPreference = 'Stop'
function Send-EnvironmentChange {
${function:Send-EnvironmentChange}
}
function Clear-PathEntry {
${function:Clear-PathEntry}
}
function Clear-Leftover {
${function:Clear-Leftover}
}
foreach (`$leftover in @(
$items
)) {
    try {
        Clear-Leftover -Leftover `$leftover
    }
    catch {
        Write-Warning `$_
    }
}
"@
}

function Invoke-ElevatedCleanup {
    # Removes leftovers that need administrator rights from an elevated
    # PowerShell, which Windows asks the user to allow (UAC)
    param([object[]]$Leftover)

    $elevatedScript = Get-ElevatedCleanupScript -Leftover $Leftover
    $scriptPath =Join-Path ([System.IO.Path]::GetTempPath()) "dtvem-uninstall-$([guid]::NewGuid()).ps1"
    [System.IO.File]::WriteAllText($scriptPath, $elevatedScript, [System.Text.Encoding]::UTF8)
    try {
        $shell = if ($PSVersionTable.PSEdition -eq 'Core') { Join-Path $PSHOME 'pwsh.exe' } else { Join-Path $PSHOME 'powershell.exe' }
        Start-Process -FilePath $shell -Verb RunAs -Wait -ArgumentList '-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', "`"$scriptPath`""
    }
    finally {
        Remove-Item -LiteralPath $scriptPath -Force -ErrorAction SilentlyContinue
    }
}

function Uninstall-DtvemRoot {
    # Removes a root (see Get-RootEntry), and reports whether it is gone
    param([string]$Root)

    foreach ($path in @(Get-RootEntry -Root $Root)) {
        if (Test-Path -LiteralPath $path -PathType Container) {
            # rd removes read-only files and paths longer than 260 characters,
            # and removes a junction or symlink (such as an `npm link`)
            # without deleting what it points to. Remove-Item in Windows
            # PowerShell 5.1 can follow a junction and empty its target.
            $output = & cmd.exe /d /c "rd /s /q `"$path`" 2>&1"
            if (Test-Path -LiteralPath $path) {
                $output | Select-Object -First 5 | ForEach-Object { Write-Host "    $_" }
            }
        }
        else {
            try {
                Remove-Item -LiteralPath $path -Force
            }
            catch {
                Write-Host "    $($_.Exception.Message)"
            }
        }
    }

    if (-not (Test-OwnedRoot -Root $Root)) {
        foreach ($dir in (Join-Path $Root 'bin'), $Root) {
            if ((Test-Path -LiteralPath $dir) -and -not (Get-ChildItem -LiteralPath $dir -Force)) {
                Remove-Item -LiteralPath $dir -Force -ErrorAction SilentlyContinue
            }
        }
    }

    return @(Get-RootEntry -Root $Root).Count -eq 0
}

function Confirm-Uninstall {
    try {
        $reply = Read-Host 'Remove dtvem? [y/N]'
    }
    catch {
        Write-Error-Custom "Could not ask for confirmation: $($_.Exception.Message)"
        Write-Info 'To uninstall without being asked, run:'
        Write-Host '    iex "& { $(irm https://raw.githubusercontent.com/CodingWithCalvin/dtvem.cli/main/uninstall.ps1) } -Yes"'
        throw 'dtvem was not uninstalled'
    }
    Write-Host ""
    return $reply -match '^\s*(y|yes)\s*$'
}

function Show-Untouched {
    # Reports dtvem-related things this script leaves alone
    $others = @(Get-Command dtvem -CommandType Application -All -ErrorAction SilentlyContinue | ForEach-Object { $_.Source })
    if ($others.Count -gt 0) {
        Write-Host ""
        Write-Warning-Custom "Another dtvem is still on your PATH: $($others -join ', ')"
        Write-Info "It wasn't installed by the dtvem installer. Remove it the way you installed it."
    }

    $variables = @()
    $userKey = [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey('Environment')
    if ($userKey) {
        $variables += @($userKey.GetValueNames() | Where-Object { $_ -like 'DTVEM_*' } | ForEach-Object { "$_ (User)" })
        $userKey.Close()
    }
    $systemKey = [Microsoft.Win32.Registry]::LocalMachine.OpenSubKey('SYSTEM\CurrentControlSet\Control\Session Manager\Environment')
    if ($systemKey) {
        $variables += @($systemKey.GetValueNames() | Where-Object { $_ -like 'DTVEM_*' } | ForEach-Object { "$_ (System)" })
        $systemKey.Close()
    }
    if ($variables.Count -gt 0) {
        Write-Host ""
        Write-Warning-Custom "These environment variables are still set: $($variables -join ', ')"
        Write-Info "Remove them in System Properties > Environment Variables if you no longer need them."
    }
}

function Invoke-DtvemUninstall {
    param([switch]$Yes)

    $ErrorActionPreference = 'Stop'

    Write-Host ""
    Write-Host "========================================" -ForegroundColor Blue
    Write-Host "   dtvem uninstaller" -ForegroundColor Blue
    Write-Host "========================================" -ForegroundColor Blue
    Write-Host ""

    $roots = @(Get-DtvemRoot)
    $leftovers = @(Find-Leftover -Roots $roots)

    if (($roots.Count -eq 0) -and ($leftovers.Count -eq 0)) {
        Write-Success "dtvem is not installed: there is nothing to remove"
        Show-Untouched
        Write-Host ""
        return
    }

    Write-Info "This will remove:"
    Write-Host ""
    foreach ($root in $roots) {
        if (Test-OwnedRoot -Root $root) {
            Write-Host "  $root"
        }
        else {
            Write-Host "  dtvem files in ${root}:"
            Get-RootEntry -Root $root | ForEach-Object { Write-Host "    $_" }
        }
        $runtimes = @(Get-InstalledRuntime -Root $root)
        if ($runtimes.Count -gt 0) {
            Write-Host "    Installed runtimes, with their global packages:"
            $runtimes | ForEach-Object { Write-Host "      $_" }
        }
        Write-Host ""
    }
    if ($leftovers.Count -gt 0) {
        foreach ($leftover in $leftovers) {
            $note = if ($leftover.Admin) { ' (needs administrator rights)' } else { '' }
            Write-Host "  $($leftover.Label)$note"
        }
        Write-Host ""
    }

    if (-not $Yes -and -not (Confirm-Uninstall)) {
        Write-Info "Canceled: nothing was removed"
        return
    }

    $failed = $false

    # A directory can't be removed while it's the working directory
    $here = (Get-Location).ProviderPath
    foreach ($root in $roots) {
        if ($here -and (Test-UnderDirectory -Path $here -Directory $root)) {
            Set-Location -LiteralPath $env:USERPROFILE
            [Environment]::CurrentDirectory = $env:USERPROFILE
            Write-Info "Changed directory to $env:USERPROFILE, out of $root"
        }
    }

    foreach ($root in $roots) {
        if (-not (Uninstall-DtvemRoot -Root $root)) {
            Write-Error-Custom "Could not remove everything in $root"
            Write-Info "Close any program using it (such as a running node, python or ruby) and run the uninstaller again."
            $failed = $true
        }
        elseif (Test-Path -LiteralPath $root) {
            Write-Success "Removed dtvem from $root (other files in it were left alone)"
        }
        else {
            Write-Success "Removed $root"
        }
    }

    $isAdmin = Test-Administrator
    $elevate = @()
    foreach ($leftover in $leftovers) {
        if ($leftover.Admin -and -not $isAdmin) {
            $elevate += $leftover
            continue
        }
        try {
            Clear-Leftover -Leftover $leftover
        }
        catch {
            Write-Host "    $($_.Exception.Message)"
        }
    }
    if ($elevate.Count -gt 0) {
        Write-Info "Asking for administrator rights to remove the rest..."
        try {
            Invoke-ElevatedCleanup -Leftover $elevate
        }
        catch {
            Write-Warning-Custom "$($_.Exception.Message)"
        }
    }

    if ($leftovers.Count -gt 0) {
        $remaining = @(Find-Leftover -Roots $roots | ForEach-Object { $_.Label })
        $notRemoved = @($leftovers | Where-Object { $remaining -contains $_.Label })
        foreach ($leftover in $leftovers) {
            if ($notRemoved -contains $leftover) {
                Write-Error-Custom "Could not remove the $($leftover.Label)"
            }
            else {
                Write-Success "Removed the $($leftover.Label)"
            }
        }
        if ($notRemoved.Count -gt 0) {
            Write-Info "Run the uninstaller again from an administrator PowerShell to remove the rest."
            $failed = $true
        }
    }

    # This PowerShell session's PATH, so dtvem is gone from it right away
    $sessionPath = @($env:Path -split ';' | Where-Object { -not (Test-DtvemLocation -Path $_ -Roots $roots) }) -join ';'
    if ($sessionPath -ne $env:Path) {
        $env:Path = $sessionPath
    }

    Show-Untouched

    Write-Host ""
    if ($failed) {
        throw 'dtvem was not completely removed. Fix the errors above and run the uninstaller again.'
    }

    Write-Host "========================================" -ForegroundColor Green
    Write-Host "   dtvem has been uninstalled" -ForegroundColor Green
    Write-Host "========================================" -ForegroundColor Green
    Write-Host ""
    Write-Info "Restart other open terminals so the PATH change takes effect there."
    Write-Info "Version files in your projects (.dtvem\runtimes.json, .tool-versions, .nvmrc,"
    Write-Info ".python-version, .ruby-version) were left in place."
    Write-Host ""
}

Invoke-DtvemUninstall -Yes:$Yes
