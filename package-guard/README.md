# Package guard

This package installs APT and dpkg hooks that protect selected package names
from removal and replacement operations.

The shared policy lives in `/etc/deepin/package-guard/`:

- `guard.conf` controls the feature with `[guard] enabled = true` or `false`.
- `protected-packages.list` contains one package name per line.

APT invokes `/usr/libexec/lastore-daemon/apt-package-guard` through
`/etc/apt/apt.conf.d/80deepin-package-guard`. Direct dpkg operations invoke
`/usr/libexec/lastore-daemon/dpkg-package-guard` through
`/etc/dpkg/dpkg.cfg.d/deepin-package-guard`.

Both hooks reload the policy for every invocation. Invalid policy files cause
that invocation to skip protection with a warning. Invalid package-list lines
are skipped individually. Once a valid non-empty policy is loaded, malformed
APT input, dpkg arguments, package metadata, or status queries are rejected.

The default list protects `lightdm`, `dde-shell`, `dde-desktop`, and `redshift`.
Edit the conffile list for the target product. Setting `enabled = false` is the
normal way to temporarily disable both hooks while retaining their registration.

Run the isolated regression suite with:

```sh
make package-guard-test
```
