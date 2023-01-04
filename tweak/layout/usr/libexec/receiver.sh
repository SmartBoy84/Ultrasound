# put whatever you want in here
if [ ! -z "$1" ]; then
    
    if [[ "$1" -eq "0" ]] ; then
        # do something if deactivate signal was sent
    fi
    
    if [[ "$1" -eq "1" ]] ; then
        # do something if activate signal was sent
        activator send libactivator.system.vibrate
    fi
    
fi