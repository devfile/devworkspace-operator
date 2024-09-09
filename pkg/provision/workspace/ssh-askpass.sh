#!/bin/sh
PASSPHRASE_FILE_PATH="/etc/ssh/passphrase"
if [ ! -f $PASSPHRASE_FILE_PATH ]; then
    echo "Error: passphrase file is missing in the '/etc/ssh/' directory" 1>&2
    exit 1
fi
cat $PASSPHRASE_FILE_PATH
