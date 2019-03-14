#!/bin/bash
#

LIBS="/usr/lib64/libz.so.1 /usr/lib64/libgcc_s.so.1"
for lib in $LIBS
do
    fullpath=$(readlink -f $lib)
    destination=$1/$(basename $lib)
    mkdir -p $(dirname $destination)
    cp -vf $fullpath $destination
done
