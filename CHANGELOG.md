<a name="0.14.7"></a>
## 0.14.7 (2019-06-19)


#### Features

*   check server Last-Modified ([08b7bc04](https://github.com/linuxdeepin/lastore-daemon/commit/08b7bc049039a564d5364a759deb3a28a83c9880))
*   remove appstore backend ([b564308c](https://github.com/linuxdeepin/lastore-daemon/commit/b564308c9ab116944338845e56a3e1cbd8bf4b12))
*   get deb app list from metadata ([14ed19ae](https://github.com/linuxdeepin/lastore-daemon/commit/14ed19aee47dfd770d330e8ca7260b5fb152dfb0))

#### Bug Fixes

*   add missing depends golang-github-go-ini-ini-dev ([f8458690](https://github.com/linuxdeepin/lastore-daemon/commit/f8458690e1a3c7bf3ca940d83490054803086e2f))
*   add If-Modified-Since ([80cf8013](https://github.com/linuxdeepin/lastore-daemon/commit/80cf8013d7edecadb1d07343ce809103988124fc))



<a name="0.14.6"></a>
### 0.14.6 (2019-05-27)


#### Bug Fixes

*   pkgNameRegexp ([40393956](https://github.com/linuxdeepin/lastore-daemon/commit/40393956abe70d4d6133d1e90a0f444a721b6534))



<a name="0.14.5"></a>
### 0.14.5 (2019-05-09)




<a name="0.14.4"></a>
### 0.14.4 (2019-04-25)




<a name="0.14.3"></a>
### 0.14.3 (2019-04-24)


#### Bug Fixes

*   update_metadata_info exit with failure ([ab8dd917](https://github.com/linuxdeepin/lastore-daemon/commit/ab8dd9176bbec7292c9de9149c6e6ce6aee5c8c2))
*   can not get upgradeable packages after update source job is end ([20b3c930](https://github.com/linuxdeepin/lastore-daemon/commit/20b3c93062da7e7f5d7e6725a7108a38deec1328))

#### Features

*   check package name ([96ac96f9](https://github.com/linuxdeepin/lastore-daemon/commit/96ac96f9a04321f36c3c6b377b657f7b870135ab))



<a name="0.14.2"></a>
### 0.14.2 (2019-04-16)


#### Bug Fixes

*   SystemUpgradeInfo return error ([040652aa](https://github.com/linuxdeepin/lastore-daemon/commit/040652aae699ab435c93e1e816a9fec39e6b681b))
*   use custom mirror failed ([5710d6f8](https://github.com/linuxdeepin/lastore-daemon/commit/5710d6f8d4765f16f65305a0316b9065a6b02085))



<a name="0.14.1"></a>
### 0.14.1 (2019-03-01)




<a name="0.14.0"></a>
## 0.14.0 (2019-02-25)


#### Bug Fixes

*   update was not found, but prompted to check for updates failed ([3a4a0385](https://github.com/linuxdeepin/lastore-daemon/commit/3a4a03852b4de80cbff79bd1028c6b0eb76113d4))

#### Features

*   add job error type unauthenticatedPackages ([0e31cf25](https://github.com/linuxdeepin/lastore-daemon/commit/0e31cf251aa53c98bc269912f4635fbe0ef1bb5d))
*   handle errors encountered when listing updateable packages ([38d41760](https://github.com/linuxdeepin/lastore-daemon/commit/38d41760de392f392fa80b1ed12f6cdffcc069b8))
*   add signal and daemon mode ([2f90113f](https://github.com/linuxdeepin/lastore-daemon/commit/2f90113f2f98d71b91c23d51cbd9fd00a9b8d171))



<a name="0.13.0"></a>
## 0.13.0 (2018-12-18)




<a name="0.12.0"></a>
## 0.12.0 (2018-12-13)


#### Features

*   save log to file ([a838246d](https://github.com/linuxdeepin/lastore-daemon/commit/a838246dd2068ecaa3b34da2267dacc7501ad721))
*   support enable config ([5676a538](https://github.com/linuxdeepin/lastore-daemon/commit/5676a5388b940e99e1f24efeb283fe88ccd808fa))
*   add adjust delay ([358506dc](https://github.com/linuxdeepin/lastore-daemon/commit/358506dc256b81be4c3becb6ab8bd7bd7b29989f))
*   add status report ([5fac1677](https://github.com/linuxdeepin/lastore-daemon/commit/5fac1677bcb21fda2c304e6e743f32aeb6d3a991))
*   support multi url dectect ([8ec08671](https://github.com/linuxdeepin/lastore-daemon/commit/8ec086711aba0c7f636829642340fc0a15b7fcd0))
*   add com.deepin.lastore.Smartmirror ([031e2d97](https://github.com/linuxdeepin/lastore-daemon/commit/031e2d97e0143dad807c11a2830e9b9d2c3f6c9f))

#### Bug Fixes

*   load mirrors.json from locale ([6e646ec2](https://github.com/linuxdeepin/lastore-daemon/commit/6e646ec2995ee2ba9facec277877b87f63eb1211))
*   dbus not auto start ([d384bd1a](https://github.com/linuxdeepin/lastore-daemon/commit/d384bd1a6be0dd9a17696c47fb211dff9943be39))
*   its Cancelable property becomes true at the end of the job ([0c0e2f91](https://github.com/linuxdeepin/lastore-daemon/commit/0c0e2f9199bf05f66586af32d419996ead1fa908))
*   ostree metadata not updated ([494aad22](https://github.com/linuxdeepin/lastore-daemon/commit/494aad22861c1f94ba73fef6273ceb66524028a3))
*   reduce query data size ([99e163b9](https://github.com/linuxdeepin/lastore-daemon/commit/99e163b9ab05b711bb0742c3279cd979efe66e77))
*   build failed with go 1.7 ([1ce7e828](https://github.com/linuxdeepin/lastore-daemon/commit/1ce7e828605a103138005ab34bc0d52b34530a5f))
*   crash on no network ([910a7989](https://github.com/linuxdeepin/lastore-daemon/commit/910a7989bdf65ba3814b32e6930391f90e421f56))
*   remove network test ([6f283108](https://github.com/linuxdeepin/lastore-daemon/commit/6f2831081d9e52ba852c0fd741eec050e914990d))



<a name="0.11.0"></a>
## 0.11.0 (2018-11-01)




<a name="0.10.0"></a>
## 0.10.0 (2018-10-25)


#### Features

*   get app metadata from new api ([6962ee45](https://github.com/linuxdeepin/lastore-daemon/commit/6962ee45c57b70082307788005705d24df096e86))
*   get mirror sources in a new way ([3dab5df5](https://github.com/linuxdeepin/lastore-daemon/commit/3dab5df5db96d664bf48616f10f917a00fcbad05))
*   add Priority set ([59905303](https://github.com/linuxdeepin/lastore-daemon/commit/59905303b8f15701be68200366d5edcc09dc8e5f))
* **daemon:**  separate update source and update metadata ([9019dc88](https://github.com/linuxdeepin/lastore-daemon/commit/9019dc88c76936b2e0bb0c92be338db8793889ae))

#### Performance

* **smartmirror:**  strip path string from mirror field ([20c9ca99](https://github.com/linuxdeepin/lastore-daemon/commit/20c9ca99c3128168dd6e47f2adff0efb0a291d5e))

#### Bug Fixes

*   generate machine-id is same ([d9c9469b](https://github.com/linuxdeepin/lastore-daemon/commit/d9c9469b627bc3357280ab152ce3f114fd1b0bf6))
* **lastore-tools:**  querydesktop deepin-fpapp-org.deepin.flatdeb.deepin-calendar no result ([4b429d36](https://github.com/linuxdeepin/lastore-daemon/commit/4b429d36c65ea712b12fe92076d854464ce755ec))



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



