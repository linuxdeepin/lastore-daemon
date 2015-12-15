#!/bin/bash

source=/var/lib/apt/lists/
target=/var/lib/lastore/safecache/lists/

mkdir -p ${target}

# clean old list files
cd $target
find . -type f |
while read fname
do
    #echo "Check... ${fname}"
    if [[ ! -f ${source}/${fname} ]]; then
	echo "remove old file: ${target}${fname}"
	rm -rf ${target}${fname}
    fi
done

# update list files
cp -vu ${source}/* $target
cp -vu /var/cache/apt/pkgcache.bin /var/lib/lastore/safecache/
