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

clean:
	rm -rf bin
	rm ../lastore-daemon_* -rf
