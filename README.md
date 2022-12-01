# Lastore Daemon

Lastore Daemon is based on dbus and support apt backend (And Currently only support apt).
The project is mainly support Linux Application Store. Currently it power deepin store 4.0.

## Dependencies
You can also check the "Depends" provided in the debian/control file.

### Build dependencies
You can also check the "Build-Depends" provided in the debian/control file.

### Runtime dependencies
- apt-get
- apt-cache
- apt

## Installation

### Debian 8.0 (jessie)

Install prerequisites
```
$ sudo apt-get install make \
                       golang-go \
                       pkg-config \
                       libglib2.0-dev
```

Build
```
$ make
```

```
$ sudo make install
```

Or, generate package files and install Deepin Terminal with it
```
$ debuild -uc -us ...
$ sudo dpkg -i ../lastore-daemon*deb
```

## Usage

### lastore-daemon and lastore-session-helper

lastore-daemon need run as root user. It will autostart by systemd
as system service.

And the lastore-session-helper need run as current session owner user.
It will autostart by dbus-daemon as session service.

There has two group of interface.
The Manager and the Updater. see [Hacking guide] for Detail information.

Normal you don't need use this. Just run deepin-appstore.

But you can use it by tools like d-feet, dbus-send or busctl.

For example, use the PackageDesktopPath api to query the desktop path of
any installed package.

```
dbus-send --print-reply --system --dest=org.deepin.dde.Lastore1 /org/deepin/dde/Lastore1 org.deepin.dde.Lastore1.Manager.PackageDesktopPath string:"gedit"
```

### lastore-tools
lastore-tools is used generate some index file in /var/lib/lastore
```
% lastore-tools -h
NAME:
   lastore-tools - help building dstore system.

USAGE:
   lastore-tools [global options] command [command options] [arguments...]

VERSION:
   0.9.18

COMMANDS:
   update	Update appstore information from server
   test		Run test job using lastore-daemon
   help, h	Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --debug, -d		show verbose message
   --help, -h		show help
   --version, -v	print the version
```

The update has two group job.
The first group jobs are pull data from server. lastore-daemon will use this to update meta data.
- categories
- applications
- xcategories
- mirrors

And the second group jobs are update data when local system changed. dpkg hook will use this to update meta data.
- desktop
- update_infos

*NOTE*: Don't use lastore-remove and lastore-install jobs. The is just for internal testing .
It will install or remove  *ALL OF APPLICATIONS* in dstore, So it very likely to broke your system.


### lastore-smartmirror
It's the helper utils for apt with smartmirror patch. Can't be used alone.

## Build Configure for mirrors

1. _./var/lib/lastore/mirrors.json_

  The package maintainer should rewrite the content if the initial values
  doesn't match the target system.

  Note:
  - It's safely remove this file from package.
  - It will update automatically at runtime by configuration.


2. _./var/lib/lastore/repository\_info.json_

  The lastore-tools will parse this file, according "/etc/apt/sources.list".

  This file support three field:
  1. *name* : the repository name. It will be send to server when update mirror lists.
  2. *url* : the official repository url which will be parsed with sources.list and get the correct *name*
  3. *mirror* (optional): the default mirror url


3. disable automatically notify system upgrade information
  1. rm /var/lib/lastore/update_infos.json
  2. run `systemctl mask lastore-build-system-info.service` for disable notify upgrade information.
  2. (DON'T DO THIS) run `systemctl mask lastore-update-metadata-info.timer` for disable update lastore metadata infos


## Getting help

Any usage issues can ask for help via

* [Gitter](https://gitter.im/orgs/linuxdeepin/rooms)
* [IRC channel](https://webchat.freenode.net/?channels=deepin)
* [Forum](https://bbs.deepin.org)
* [WiKi](http://wiki.deepin.org/)

## Getting involved

We encourage you to report issues and contribute changes

* [Contribution guide for users](http://wiki.deepin.org/index.php?title=Contribution_Guidelines_for_Users)
* [Contribution guide for developers](http://wiki.deepin.org/index.php?title=Contribution_Guidelines_for_Developers).
* [How contributing this project](HACKING.org)

## License

Lastore Daemon is licensed under [GPL-3.0-or-later](LICENSE).
