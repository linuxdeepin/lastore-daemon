all:  build


build: 
	GOPATH=`pwd`:`pwd`/vendor go build -o bin/lastore-daemon lastore-daemon

install:
	mkdir -p ${DESTDIR}${PREFIX}/usr/bin && cp bin/lastore-daemon ${DESTDIR}${PREFIX}/usr/bin/
	mkdir -p ${DESTDIR}${PREFIX}/usr && cp -rf usr ${DESTDIR}${PREFIX}/
	cp -rf etc ${DESTDIR}${PREFIX}/etc



gen-xml:
	qdbus --system org.deepin.lastore /org/deepin/lastore org.freedesktop.DBus.Introspectable.Introspect > usr/share/dbus-1/interfaces/org.deepin.lastore.xml
	qdbus --system org.deepin.lastore /org/deepin/lastore/Job1 org.freedesktop.DBus.Introspectable.Introspect > usr/share/dbus-1/interfaces/org.deepin.lastore.Job.xml
gen-dbus-codes:
	~/prj/dbus-generator/dbus-generator -o usr/include/lastore-daemon.h usr/share/dbus-1/interfaces/*.xml


build-deb:
	yes | debuild -us -uc

upload:
	scp -r ../lastore-daemon_*.deb snyh@10.0.4.226:/repos/mirror/dev/tmp/
	ssh snyh@10.0.4.226 'cd /repos/mirror/dev/ && reprepro includedeb unstable tmp/*.deb'

clean:
	rm -rf bin
