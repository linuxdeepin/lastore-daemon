#!/bin/sh

mkdir -p /var/lib/lastore/safecache/lists
cp -vu /var/lib/apt/lists/* /var/lib/lastore/safecache/lists
cp -vu /var/cache/apt/pkgcache.bin /var/lib/lastore/safecache/
apt-cache gencaches -c /var/lib/lastore/apt.conf
