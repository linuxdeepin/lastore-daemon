#!/bin/bash
# /var/cache/apt/pkgcache.bin will be deleted temporarily
# whenever executing "apt-get update".
tmpfile=$(mktemp /var/lib/lastore/safecache_pkgcache.bin.XXXXXX)
cp -fv /var/cache/apt/pkgcache.bin "$tmpfile"
mv -f "$tmpfile" /var/lib/lastore/safecache_pkgcache.bin
