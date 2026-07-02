#!/bin/sh
# Only handle passphrase prompts - exit silently for other prompts (e.g., HTTPS git repo username/password request)
case "$1" in
  "Enter passphrase for key '"*) ;;
  *) exit 0 ;;
esac
PASSPHRASE_FILE_PATH="/etc/ssh/passphrase"
if [ ! -f $PASSPHRASE_FILE_PATH ]; then
    echo "Error: passphrase file is missing in the '/etc/ssh/' directory" 1>&2
    exit 1
fi
cat $PASSPHRASE_FILE_PATH
