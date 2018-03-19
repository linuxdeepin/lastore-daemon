<a name="0.9.55"></a>
### 0.9.55 (2018-03-19)


#### Bug Fixes

*   failed create hardlink if the parenter directory miss ([e7d91ebb](https://github.com/linuxdeepin/lastore-daemon/commit/e7d91ebb520a00801d747d23d5371c6b4166ca2d))



<a name="0.9.54"></a>
### 0.9.54 (2018-02-26)


#### Performance

*   reduce RAM usage by avoid contents of safecache ([1028f5c4](https://github.com/linuxdeepin/lastore-daemon/commit/1028f5c44b90cf9aa19c5ea0712b0b1fecc91468))
*   run lastore-daemon by needing ([12001a17](https://github.com/linuxdeepin/lastore-daemon/commit/12001a172280f52bb78218e5c7b86563ac50a63c))

#### Bug Fixes

*   build_system_info exit if UPDATE_INFO time invalid ([95881da4](https://github.com/linuxdeepin/lastore-daemon/commit/95881da41545cfafd28fd122bc320a933037ff28))
* **apt-clean:**  can not delete some files ([6916eb3c](https://github.com/linuxdeepin/lastore-daemon/commit/6916eb3ca24e4ca2d31d270e306b1db6ecb1b8f9))
* **autoclean:**  fix calcRemainingDuration ([7ea8126c](https://github.com/linuxdeepin/lastore-daemon/commit/7ea8126ca12fbb146d47444e9a1301fa3f1e7382))

#### Features

* **apt-clean:**  carefully handle status of package ([0e7318ff](https://github.com/linuxdeepin/lastore-daemon/commit/0e7318ffa818fe504c8c973372141c2aaee35bc5))
* **session-helper:**  check system source ([e6660c88](https://github.com/linuxdeepin/lastore-daemon/commit/e6660c88ce2e6d2fa780b60fa5ecff6dab809576))



<a name="0.9.53"></a>
### 0.9.53 (2017-12-06)


#### Bug Fixes

*   race condition JobManager.changed ([4b974d5f](https://github.com/linuxdeepin/lastore-daemon/commit/4b974d5f27fac8ec3d128ab9de071e0c262ba0f7))
*   don't Install dbus object when testing ([4d8e7b17](https://github.com/linuxdeepin/lastore-daemon/commit/4d8e7b177948f8de97a244f4d884262693eb5f52))
*   race condition vendor/ dbus's HandleNewMessage ([1fa4e3f7](https://github.com/linuxdeepin/lastore-daemon/commit/1fa4e3f777aba912583185c563082596665e1a77))
*   race condition on JobQueue ([8c4c0f2a](https://github.com/linuxdeepin/lastore-daemon/commit/8c4c0f2a6c47c65cc61d00ee92fd71d6eeed1408))



<a name="0.9.52"></a>
### 0.9.52 (2017-11-24)


#### Performance

*   don't build cache if both dpkg && apt hasn't any changes. ([6260ab52](https://github.com/linuxdeepin/lastore-daemon/commit/6260ab52e28de70633a18713fd23791adf5f6f8c))

#### Bug Fixes

*   build_system_info ignore executing if lists only change ctime ([3b72fb28](https://github.com/linuxdeepin/lastore-daemon/commit/3b72fb287358df7f8c28936d6000e3cacbe10ef5))



<a name="0.9.51"></a>
### 0.9.51 (2017-11-20)


#### Features

*   add --allow-change-held-packages option for DownloadJobType ([5b2585f5](https://github.com/linuxdeepin/lastore-daemon/commit/5b2585f52290ee9b80b92f1a57c1a1959fe3c19c))

#### Bug Fixes

*   "lastore-tools querydesktop firefox-dde" failed ([060822c1](https://github.com/linuxdeepin/lastore-daemon/commit/060822c109c663037532223c17b80b6293228c8a))



<a name="0.9.50"></a>
### 0.9.50 (2017-11-17)


#### Bug Fixes

*   lastore-apt-clane panic if deb file name is abnormal ([927f36ee](https://github.com/linuxdeepin/lastore-daemon/commit/927f36ee0e4d4d6ebf1bacd83c252e672dfa11c5))

#### Features

*   add lastore-apt-clean tool ([d13a2bd1](https://github.com/linuxdeepin/lastore-daemon/commit/d13a2bd1f0a97c656df583d1b438ddf4f7ec97b4))



<a name="0.9.49"></a>
### 0.9.49 (2017-11-09)

#### Features
*   QueryDesktopFile support deepin flatpak app package ([0b8821f8](https://github.com/linuxdeepin/lastore-daemon/commit/0b8821f8993c410e502ce6e85e172d652e285064))


<a name="0.9.47"></a>
### 0.9.47 (2017-10-17)


#### Features

*   use gnome debconf frontend to avoid blocking ([aba52c10](https://github.com/linuxdeepin/lastore-daemon/commit/aba52c10d3497951980a6afa91304b40a39cd24c))


<a name="0.9.46"></a>
### 0.9.46 (2017-09-14)

#### Features

*  (lastore-tools) add command of querydesktop ([cb3269f4](https://github.com/linuxdeepin/lastore-daemon/commit/cb3269f49fb8739c003a08f9c65ec6f837bf98b0))


#### Bug Fixes

*   unify LICENSE header ([e8ca7e53](https://github.com/linuxdeepin/lastore-daemon/commit/e8ca7e536ff8125695ec278cace25d8a9d61abb7))

#### Others

*   configure clog for automatically  generating CHANGELOG.md ([5f33c913](https://github.com/linuxdeepin/lastore-daemon/commit/5f33c91307ef8367a17c96ea85c1cc4b1b6fcdc2))



