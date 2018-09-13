<a name="0.9.66.3"></a>
## 0.9.66.3 (2018-09-13)


#### Bug Fixes

*   generate machine-id is same ([79e1b811](https://github.com/linuxdeepin/lastore-daemon/commit/79e1b811cc8811cbdc49c91462911fed9484c97c))
* **lastore-tools:**  querydesktop deepin-fpapp-org.deepin.flatdeb.deepin-calendar no result ([ad88eec3](https://github.com/linuxdeepin/lastore-daemon/commit/ad88eec34e8da5f082265eab20694935103544ad))

#### Features

* **daemon:**  separate update source and update metadata ([abffdf2b](https://github.com/linuxdeepin/lastore-daemon/commit/abffdf2b079676497b42a57bdce010cd3448d821))



<a name="0.9.66.2"></a>
## 0.9.66.2 (2018-08-30)


#### Features

*   add Priority set ([ec4dc849](https://github.com/linuxdeepin/lastore-daemon/commit/ec4dc84968a79d805c304862d2bccc9b66461bc3))



<a name="0.9.66.1"></a>
### 0.9.66.1 (2018-08-30)


#### Bug Fixes

*   after upgrade, click restart bring up an authorization dialog ([0b614b58](https://github.com/linuxdeepin/lastore-daemon/commit/0b614b5865d027d972046d27674e1e05bd9c4a2f))



<a name="0.9.66"></a>
### 0.9.66 (2018-08-12)


#### Bug Fixes

*   `sw_64` gccgo720 build failed ([388d9199](https://github.com/linuxdeepin/lastore-daemon/commit/388d919910dd7bf8ddc8bf8f6640ba87a3ffddb9))
* **lastore-tools:**  no check categories api return value structure ([31dbffd3](https://github.com/linuxdeepin/lastore-daemon/commit/31dbffd3a38e8d026f8e4b04b132dc070c0d1d55))



<a name="0.9.65"></a>
### 0.9.65 (2018-07-20)




<a name="0.9.64"></a>
### 0.9.64 (2018-07-19)




<a name="0.9.63"></a>
### 0.9.63 (2018-07-19)


#### Features

*   clean archives from UI do not send notification ([8e642a1d](https://github.com/linuxdeepin/lastore-daemon/commit/8e642a1d499afe453450965a7229ff902669f8dc))
* **daemon:**  handle more errors ([72660dbc](https://github.com/linuxdeepin/lastore-daemon/commit/72660dbc9eec8d567de8225b38f9a70a9c6f76fb))

#### Bug Fixes

*   PkgSystemError.GetType typo ([7353f307](https://github.com/linuxdeepin/lastore-daemon/commit/7353f307479f5b32867b9edbb50b5b7bfb535a07))



<a name="0.9.62"></a>
### 0.9.62 (2018-07-05)


#### Features

*   handle package manager system error ([ace591d9](https://github.com/linuxdeepin/lastore-daemon/commit/ace591d938558c76a24bef84f9c90cd615feba49))
* **backend-deb:**  watch lastore-daemon online/offline ([e750db42](https://github.com/linuxdeepin/lastore-daemon/commit/e750db42a0d7a3018ca492a430e5f06220faafca))

#### Performance

*   reduce CPU usage by remove defer function ([5e30c8aa](https://github.com/linuxdeepin/lastore-daemon/commit/5e30c8aa456c9721bc5f109c74dea1a02e3df093))

#### Bug Fixes

* **daemon:**
  *  some failed jobs have not been retried ([33d435dd](https://github.com/linuxdeepin/lastore-daemon/commit/33d435dd9455f8b314315c47d80558f554bdb763))
  *  property JobList occasionally lose job ([4b5f9131](https://github.com/linuxdeepin/lastore-daemon/commit/4b5f913113296d6f8aed7354b79729e3e2680ec1))
* **session-helper:**  may send failed notification more than once ([d21c537f](https://github.com/linuxdeepin/lastore-daemon/commit/d21c537f3bef8b93a1cd2e323f5bc512aff9f613))



<a name="0.9.61"></a>
### 0.9.61 (2018-06-07)


#### Features

* **backend-deb:**
  *  add method CleanArchives ([ad1ce52a](https://github.com/linuxdeepin/lastore-daemon/commit/ad1ce52acc594a82fc518d0c2e57ac450edb1a5f))
  *  ListInstalled add installed size field ([8229d85a](https://github.com/linuxdeepin/lastore-daemon/commit/8229d85abf81eff477d97e5900a9bf83fb3244da))

#### Bug Fixes

*   can't transit the status of job from ready to end ([1e6239ba](https://github.com/linuxdeepin/lastore-daemon/commit/1e6239ba872e967ee8559d246aff87aa55dd8b57))
*   TestSourceLineParsed_String test failed ([70893ac8](https://github.com/linuxdeepin/lastore-daemon/commit/70893ac87e056ad08e52b5d1b39dccd3446e5983))



<a name="0.9.60"></a>
### 0.9.60 (2018-05-30)


#### Bug Fixes

* **backend-deb:**  not listen the Status property of Job change ([4b79b9a6](https://github.com/linuxdeepin/lastore-daemon/commit/4b79b9a6522c4e65f396537cdabe6db5f28a3a89))
* **daemon:**  some data race problem ([b4433ee2](https://github.com/linuxdeepin/lastore-daemon/commit/b4433ee2ded4f22659a033d7d716b7a083fd153d))

#### Features

*   InstallPackage do not return resource exist error ([67a4cc21](https://github.com/linuxdeepin/lastore-daemon/commit/67a4cc21c80d08169eb2fd4b9379c7e5ab026ec8))
* **backend-deb:**  add method QueryInstallationTime ([cd6c3b27](https://github.com/linuxdeepin/lastore-daemon/commit/cd6c3b27d94c6633b12a1dc4e2ac830ed419d866))
* **session-helper:**
  *  check source not check sources.list.d ([03fac892](https://github.com/linuxdeepin/lastore-daemon/commit/03fac892c81a7bf026ce7223a016e061b2ff2498))
  *  check source more loosely ([a3fe8eb9](https://github.com/linuxdeepin/lastore-daemon/commit/a3fe8eb9275549b243901b3840b546ac12214527))



<a name="0.9.59"></a>
### 0.9.59 (2018-05-14)


#### Bug Fixes

*   gccgo compile error ([28480ded](https://github.com/linuxdeepin/lastore-daemon/commit/28480dedd8137096dac7ce909ba2302a62673237))

#### Features

*   add bin backend-deb ([d67e2c4b](https://github.com/linuxdeepin/lastore-daemon/commit/d67e2c4b8247d9e04072b8425b2af99782b32ded))



<a name="0.9.58"></a>
### 0.9.58 (2018-04-23)


#### Bug Fixes

* **script:**  build_system_info does not trigger update handler ([2e08ccb4](https://github.com/linuxdeepin/lastore-daemon/commit/2e08ccb41867825a3f4feb3a44dcc8d7623e4bc0))



<a name="0.9.57"></a>
### 0.9.57 (2018-04-19)


#### Bug Fixes

*   debconfig frontend locale is not set ([d6659e67](https://github.com/linuxdeepin/lastore-daemon/commit/d6659e67a27abc33c3fb738e5930e4d7e0c0746b))



<a name="0.9.56"></a>
### 0.9.56 (2018-04-12)


#### Features

* **session-helper:**  add method IsDiskSpaceSufficient ([b080ef65](https://github.com/linuxdeepin/lastore-daemon/commit/b080ef658449acec0eecd422a83c092df6a9a713))



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



