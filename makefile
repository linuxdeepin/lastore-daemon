ifndef USE_GCCGO
	GOBUILD = go build
else
	GOBUILD = go build -compiler gccgo
endif

all:  build

gb-bin:
	GOPATH=`pwd`:`pwd`/vendor ${GOBUILD} -o vendor/gb github.com/constabulary/gb/cmd/gb


bin/lastore-tools:
	GOPATH=`pwd`:`pwd`/vendor ${GOBUILD} -o bin/lastore-tools lastore-tools

build:  bin/lastore-tools
	GOPATH=`pwd`:`pwd`/vendor ${GOBUILD} -o bin/lastore-daemon lastore-daemon
	GOPATH=`pwd`:`pwd`/vendor ${GOBUILD} -o bin/lastore-session-helper lastore-session-helper
	GOPATH=`pwd`:`pwd`/vendor ${GOBUILD} -o bin/lastore-smartmirror lastore-smartmirror || echo "build failed, disable smartmirror support "

test: gb-bin
	./vendor/gb test

gb: gb-bin
	./vendor/gb build lastore-daemon
	./vendor/gb build lastore-tools
	./vendor/gb build lastore-session-helper
	./vendor/gb build lastore-smartmirror || echo "build failed, disable smartmirror support "

install: gen_mo bin/lastore-tools
	mkdir -p ${DESTDIR}${PREFIX}/usr/bin && cp bin/* ${DESTDIR}${PREFIX}/usr/bin/
	mkdir -p ${DESTDIR}${PREFIX}/usr && cp -rf usr ${DESTDIR}${PREFIX}/
	cp -rf etc ${DESTDIR}${PREFIX}/etc

	mkdir -p ${DESTDIR}${PREFIX}/var/lib/lastore/
	cp -rf var/lib/lastore/* ${DESTDIR}${PREFIX}/var/lib/lastore/
	cp -rf lib ${DESTDIR}${PREFIX}/

update_pot:
	deepin-update-pot locale/locale_config.ini

gen_mo:
	deepin-generate-mo locale/locale_config.ini
	mkdir -p ${DESTDIR}${PREFIX}/usr/share/locale/
	cp -rf locale/mo/* ${DESTDIR}${PREFIX}/usr/share/locale/

	deepin-generate-mo locale_categories/locale_config.ini
	cp -rf locale_categories/mo/* ${DESTDIR}${PREFIX}/usr/share/locale/

gen-xml:
	qdbus --system com.deepin.lastore /com/deepin/lastore org.freedesktop.DBus.Introspectable.Introspect > usr/share/dbus-1/interfaces/com.deepin.lastore.xml
	qdbus --system com.deepin.lastore /com/deepin/lastore/Job1 org.freedesktop.DBus.Introspectable.Introspect > usr/share/dbus-1/interfaces/com.deepin.lastore.Job.xml

gen-dbus-codes:
	~/prj/go-dbus-generator/go-dbus-generator -o usr/include/lastore-daemon.h usr/share/dbus-1/interfaces/*.xml

build-deb:
	yes | debuild -us -uc

clean:
	rm -rf bin
	rm -rf pkg
	rm -rf vendor/pkg
	rm -rf vendor/gb
	rm -rf vendor/bin
