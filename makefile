ifndef USE_GCCGO
	GOBUILD = go build
else
	GOBUILD = go build -compiler gccgo
endif

all:  build

bin/lastore-tools:
	GOPATH=`pwd`:`pwd`/vendor ${GOBUILD} -o bin/lastore-tools lastore-tools

build:  bin/lastore-tools
	GOPATH=`pwd`:`pwd`/vendor ${GOBUILD} -o bin/lastore-daemon lastore-daemon
	GOPATH=`pwd`:`pwd`/vendor ${GOBUILD} -o bin/lastore-session-helper lastore-session-helper
	GOPATH=`pwd`:`pwd`/vendor ${GOBUILD} -o bin/lastore-smartmirror lastore-smartmirror || echo "build failed, disable smartmirror support "

fetch-base-metadata:
	./bin/lastore-tools update -r desktop -j applications -o ${DESTDIR}${PREFIX}/var/lib/lastore/applications.json
	./bin/lastore-tools update -r desktop -j categories -o ${DESTDIR}${PREFIX}/var/lib/lastore/categories.json
	./bin/lastore-tools update -r desktop -j mirrors -o ${DESTDIR}${PREFIX}/var/lib/lastore/mirrors.json

test:
	GOPATH=`pwd`:`pwd`/vendor go test internal/system internal/system/apt \
	internal/utils	lastore-daemon  lastore-session-helper  lastore-smartmirror  lastore-tools

install: gen_mo bin/lastore-tools fetch-base-metadata
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

build-deb:
	yes | debuild -us -uc

clean:
	rm -rf bin
	rm -rf pkg
	rm -rf vendor/pkg
	rm -rf vendor/bin
