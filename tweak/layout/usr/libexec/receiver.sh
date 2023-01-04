# put whatever you want in here
if [ ! -z "$1" ]; then
    
    if [[ "$1" -eq "0" ]] ; then
        # do something if deactivate signal was sent
        echo "[goceiver] DEACTIVATE"
    fi
    
    if [[ "$1" -eq "1" ]] ; then
        # do something if activate signal was sent
        echo "[goceiver] ACTIVATE"
        activator send libactivator.system.vibrate
    fi
    
fi