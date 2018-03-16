#!/bin/bash
# /var/cache/apt/pkgcache.bin will be deleted temporarily
# whenever executing "apt-get update".
ln -fv /var/cache/apt/pkgcache.bin /var/lib/lastore/safecache_pkgcache.bin
