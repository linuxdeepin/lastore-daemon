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
find ${source} -maxdepth 1 -type f -exec cp '{}' ${target} \;
cp  /var/cache/apt/pkgcache.bin /var/lib/lastore/safecache/
