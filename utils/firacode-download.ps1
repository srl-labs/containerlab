# execute in the powershell console like this:
# iex ((New-Object System.Net.WebClient).DownloadString('https://raw.githubusercontent.com/srl-labs/containerlab/refs/heads/main/utils/firacode-download.ps1'))

# Get the ID and security principal of the current user account
$myWindowsID = [System.Security.Principal.WindowsIdentity]::GetCurrent()
$myWindowsPrincipal = new-object System.Security.Principal.WindowsPrincipal($myWindowsID)
# Get the security principal for the Administrator role
$adminRole = [System.Security.Principal.WindowsBuiltInRole]::Administrator
# Check to see if we are currently running "as Administrator"
if (!($myWindowsPrincipal.IsInRole($adminRole))) {
    # We are not running "as Administrator" - so relaunch as administrator
    
    # Create a new process object that starts PowerShell
    $newProcess = new-object System.Diagnostics.ProcessStartInfo "PowerShell";
    
    # Specify the current script path and name as a parameter
    $newProcess.Arguments = $myInvocation.MyCommand.Definition;
    
    # Indicate that the process should be elevated
    $newProcess.Verb = "runas";
    
    # Start the new process
    [System.Diagnostics.Process]::Start($newProcess);
    
    # Exit from the current, unelevated, process
    exit
} 

$userProfile = $env:userprofile
$tmpDirName = "clab_tmp_font"
$tmpDirPath = $($userProfile + "\" + $tmpDirName)

# Write-Host($userProfile + "\" + $tmpDirName)
# Write-Host(Test-Path -PathType Container $tmpDirPath)

# create tmp dir, if it doesn't exist
if (!(Test-Path -PathType Container $tmpDirPath)) {
    New-Item -Path $userProfile -Name $tmpDirName -ItemType "directory" -Force
}

# set location to the tmp dir
Set-Location $tmpDirPath

# Download font
Invoke-WebRequest "https://github.com/ryanoasis/nerd-fonts/releases/latest/download/FiraCode.zip" -UseBasicParsing -OutFile ".\FiraCode.zip"

# unzip
Expand-Archive "FiraCode.zip" -DestinationPath ".\FiraCode" -Force

# Populate list of all font files
$Fonts = Get-ChildItem -Path ".\FiraCode" -Recurse -Include "*.ttf"

$fontWinDir = $Env:WinDir + "\Fonts"

foreach ($Font in $Fonts) {

    # create tmp dir, if it doesn't exist
    if (!(Test-Path ($fontWinDir + "\" + $Font.name))) {
        Write-Host("Copying $Font to $fontWinDir") -ForegroundColor "Green"
        Copy-Item $Font $Env:WinDir\Fonts
    }
    else {
        Write-Host($Font.name + " already exists, don't need to copy.") -ForegroundColor "Red"
    }

    # check if the key exists
    if ((Get-ItemProperty "HKLM:\Software\Microsoft\Windows NT\CurrentVersion\Fonts").PSObject.Properties.Name -contains $Font.BaseName) {
        Write-Host("Not creating registry key for " + $Font.BaseName + " already exists") -ForegroundColor "Red"
    }
    else {
        Write-Host("Creating registry key for " + $Font.BaseName) -ForegroundColor "Green"
        New-ItemProperty -Name $Font.BaseName -Path "HKLM:\Software\Microsoft\Windows NT\CurrentVersion\Fonts" -PropertyType string -Value $Font.name
    }
}

# Cleanup
Set-Location $userProfile
Remove-Item $tmpDirPath -Recurse

Write-Host("Done!")

pause