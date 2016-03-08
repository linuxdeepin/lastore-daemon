# Lastore Daemon

**Description**

[![Build Status](https://ci.deepin.io/job/lastore-daemon/badge/icon)](https://ci.deepin.io/job/lastore-daemon)

Lastore Daemon is based on dbus and support apt backend (And Currently only support apt).

The project is mainly support Linux Application Store. Currently it power deepin store 4.0.

## Dependencies

### Build dependencies
- golang >= 1.2
- pkg-config
- make
- glib-2.0

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
dbus-send --print-reply --system --dest=com.deepin.lastore /com/deepin/lastore com.deepin.lastore.Manager.PackageDesktopPath string:"gedit"
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

Lastore Daemon is licensed under [GPLv3](LICENSE).
