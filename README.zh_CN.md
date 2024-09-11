# Lastore Daemon

Lastore Daemon 基于 dbus，支持 apt 后端（目前仅支持 apt）。
该项目主要是支持Linux应用商店。 目前它为深度商店4.0提供支持。

## 依赖
请查看“debian/control”文件中提供的“Depends”。

### 编译依赖
请查看“debian/control”文件中提供的“Build-Depends”。

### 运行依赖
- apt-get
- apt-cache
- apt

## 安装

### Debian 8.0 (jessie)

确保已经安装了所有的编译依赖
```
$ sudo apt-get install make \
                       golang-go \
                       pkg-config \
                       libglib2.0-dev
```

构建
```
$ make
```

```
$ sudo make install
```

或者，生成包文件并在深度终端安装它
```
$ debuild -uc -us ...
$ sudo dpkg -i ../lastore-daemon*deb
```

## 用法

### lastore-daemon and lastore-session-helper

lastore-daemon 需要以 root 用户身份运行。 它将由 systemd 自动启动
作为系统服务。

并且 lastore-session-helper 需要以当前会话所有者用户身份运行。
它将由 dbus-daemon 作为会话服务自动启动。

有两组接口。
管理器和更新器。 有关详细信息，请参阅 [黑客指南]。

正常你不需要使用这个。 只需运行 deepin-appstore。

但是您可以通过 d-feet、dbus-send 或 busctl 等工具来使用它。

例如，使用 PackageDesktopPath api 查询桌面路径
任何已安装的软件包。

```
dbus-send --print-reply --system --dest=org.deepin.dde.Lastore1 /org/deepin/dde/Lastore1 org.deepin.dde.Lastore1.Manager.PackageDesktopPath string:"gedit"
```

### lastore-tools
lastore-tools 用于在 /var/lib/lastore 中生成一些索引文件
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

更新有两组任务。
第一组任务是从服务器拉数据。 lastore-daemon 将使用它来更新元数据。
- categories
- applications
- xcategories
- mirrors

第二组任务是本地系统更改时更新数据。 dpkg hook 将使用它来更新元数据。
- desktop
- update_infos

*注意*：不要使用 lastore-remove 和 lastore-install。 这两个任务仅用于内部测试。
它将在 dstore 中安装或删除 *所有应用*，因此很可能会破坏您的系统。


### lastore-smartmirror
它是 apt 与 smartmirror 补丁的帮助工具。 不能单独使用。

## 为镜像构建配置

1. _./var/lib/lastore/mirrors.json_

  如果初始值与目标系统不匹配，包维护者应重写内容。

  注意：
   - 它可以安全地从包中删除此文件。
   - 它会在运行时通过配置自动更新。


2. _./var/lib/lastore/repository\_info.json_

  lastore-tools 将根据“/etc/apt/sources.list”解析这个文件。

  该文件支持三个字段：
  1. *name* : 仓库名称。 更新镜像列表时发送到服务器
  2. *url* : 官方存储库 url 将使用 sources.list 解析并获得正确的 *name*
  3. *mirror* (可选): 默认镜像地址


3. 禁用自动通知系统升级信息
  1. rm /var/lib/lastore/update_infos.json
  2. 运行 `systemctl mask lastore-build-system-info.service` 禁用通知系统升级信息。
  2. (一定不要这样做) 运行`systemctl mask lastore-update-metadata-info.timer`以禁用更新lastore元数据信息


## 获得帮助

如果您遇到任何其他问题，您可能会发现这些渠道很有用：

* [Gitter](https://gitter.im/orgs/linuxdeepin/rooms)
* [IRC Channel](https://webchat.freenode.net/?channels=deepin)
* [官方论坛](https://bbs.deepin.org/)
* [Wiki](https://wiki.deepin.org/)
* [项目地址](https://github.com/linuxdeepin/lastore-daemon)

## 贡献指南

我们鼓励您报告问题并做出更改

* [Contribution guide for developers](https://github.com/linuxdeepin/developer-center/wiki/Contribution-Guidelines-for-Developers-en). (English)
* [开发者代码贡献指南](https://github.com/linuxdeepin/developer-center/wiki/Contribution-Guidelines-for-Developers) (中文)
* [How contributing this project](HACKING.org)

## License

Lastore Daemon 在 [GPL-3.0-or-later](LICENSE)下发布。
