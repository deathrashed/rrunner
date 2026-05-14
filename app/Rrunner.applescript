on open location this_URL
    set runnerPath to POSIX path of (path to home folder) & ".local/bin/rrunner"

    do shell script "/bin/mkdir -p " & quoted form of (POSIX path of (path to home folder) & ".local/bin")

    try
        do shell script "/usr/bin/test -x " & quoted form of runnerPath
    on error
        display alert "Rrunner" message "The Rrunner command-line bridge was not found at:" & return & runnerPath & return & return & "Run install.sh from the Rrunner repo."
        return
    end try

    do shell script quoted form of runnerPath & space & quoted form of this_URL & " >/tmp/Rrunner.log 2>&1 &"
end open location

on run
    display dialog "Rrunner is installed and handles rrunner:// links." buttons {"OK"} default button "OK"
end run
