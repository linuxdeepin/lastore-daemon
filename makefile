all:  build


build: 
	GOPATH=`pwd`:`pwd`/vendor go build -o bin/lastore-daemon lastore-daemon
	GOPATH=`pwd`:`pwd`/vendor go build -o bin/lastore-tools tools

install: 
	mkdir -p ${DESTDIR}${PREFIX}/usr/bin && cp bin/* ${DESTDIR}${PREFIX}/usr/bin/
	mkdir -p ${DESTDIR}${PREFIX}/usr && cp -rf usr ${DESTDIR}${PREFIX}/
	cp -rf etc ${DESTDIR}${PREFIX}/etc
	mkdir -p ${DESTDIR}${PREFIX}/var/lib/lastore/
	cp -rf var/lib/lastore/* ${DESTDIR}${PREFIX}/var/lib/lastore/



gen-xml:
	qdbus --system com.deepin.lastore /com/deepin/lastore org.freedesktop.DBus.Introspectable.Introspect > usr/share/dbus-1/interfaces/com.deepin.lastore.xml
	qdbus --system com.deepin.lastore /com/deepin/lastore/Job1 org.freedesktop.DBus.Introspectable.Introspect > usr/share/dbus-1/interfaces/com.deepin.lastore.Job.xml

gen-dbus-codes:
	~/prj/go-dbus-generator/go-dbus-generator -o usr/include/lastore-daemon.h usr/share/dbus-1/interfaces/*.xml


build-deb:
	yes | debuild -us -uc

clean:
	rm -rf bin
	rm ../lastore-daemon_* -rf

bin/lastore-tools:
	gb build -o bin/lastore-tools tools

var/lib/lastore: var/lib/lastore/application.json var/lib/lastore/categories.json var/lib/lastore/xcategories.json

var/lib/lastore/application.json: bin/lastore-tools
	mkdir -p var/lib/lastore
	./bin/lastore-tools -item applications -output $@

var/lib/lastore/categories.json: bin/lastore-tools
	mkdir -p var/lib/lastore
	./bin/lastore-tools -item categories -output  $@

var/lib/lastore/xcategories.json: bin/lastore-tools
	mkdir -p var/lib/lastore
	./bin/lastore-tools -item xcategories -output  $@
