property bridgePath : "/Users/rd/.local/bin/rrunner"

on run
	set ok to my bridge_exists()
	if ok is false then
		display dialog "The Rrunner command-line bridge was not found or is not executable at:" & return & bridgePath & return & return & "Run install.sh from the Rrunner repo." buttons {"OK"} default button "OK" with icon stop
	else
		display dialog "Rrunner is installed and ready." & return & return & bridgePath buttons {"OK"} default button "OK"
	end if
end run

on open location thisURL
	set ok to my bridge_exists()
	if ok is false then
		display dialog "The Rrunner command-line bridge was not found or is not executable at:" & return & bridgePath & return & return & "Run install.sh from the Rrunner repo." buttons {"OK"} default button "OK" with icon stop
		return
	end if
	
	do shell script quoted form of bridgePath & space & quoted form of thisURL
end open location

on bridge_exists()
	try
		do shell script "/bin/test -x " & quoted form of bridgePath
		return true
	on error
		return false
	end try
end bridge_exists