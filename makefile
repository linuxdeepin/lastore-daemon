PBUILDER_PKG = pbuilder-satisfydepends-dummy

pwd := ${shell pwd}
GoPath := GOPATH=${pwd}:${pwd}/vendor:${GOPATH}

GOBUILD = go build
GOTEST = go test -v

all:  build

build:
	${GoPath} ${GOBUILD} -o bin/lastore-daemon lastore-daemon
	${GoPath} ${GOBUILD} -o bin/lastore-tools lastore-tools
	${GoPath} ${GOBUILD} -o bin/lastore-smartmirror lastore-smartmirror || echo "build failed, disable smartmirror support "
	${GoPath} ${GOBUILD} -o bin/lastore-smartmirror-daemon lastore-smartmirror-daemon || echo "build failed, disable smartmirror support "
	${GoPath} ${GOBUILD} -o bin/lastore-apt-clean lastore-apt-clean

fetch-base-metadata:
	./bin/lastore-tools update -r desktop -j applications -o var/lib/lastore/applications.json
	./bin/lastore-tools update -r desktop -j categories -o var/lib/lastore/categories.json
	./bin/lastore-tools update -r desktop -j mirrors -o var/lib/lastore/mirrors.json


test:
	NO_TEST_NETWORK=$(shell \
	if which dpkg >/dev/null;then \
		if dpkg -s ${PBUILDER_PKG} 2>/dev/null|grep 'Status:.*installed' >/dev/null;then \
			echo 1; \
		fi; \
	fi) \
	${GoPath} ${GOTEST} internal/system internal/system/apt \
	internal/utils	internal/querydesktop \
	lastore-daemon  lastore-smartmirror  lastore-tools \
	lastore-smartmirror-daemon

test-coverage: 
	env ${GoPath} go test -cover -v ./src/... | awk '$$1 ~ "^(ok|\\?)" {print $$2","$$5}' | sed "s:${CURDIR}::g" | sed 's/files\]/0\.0%/g' > coverage.csv


print_gopath:
	GOPATH="${pwd}:${pwd}/vendor:${GOPATH}"

install: gen_mo
	mkdir -p ${DESTDIR}${PREFIX}/usr/bin && cp bin/lastore-apt-clean ${DESTDIR}${PREFIX}/usr/bin/
	cp bin/lastore-tools ${DESTDIR}${PREFIX}/usr/bin/
	cp bin/lastore-smartmirror ${DESTDIR}${PREFIX}/usr/bin/
	mkdir -p ${DESTDIR}${PREFIX}/usr/libexec/lastore-daemon && cp bin/lastore-daemon ${DESTDIR}${PREFIX}/usr/libexec/lastore-daemon
	cp bin/lastore-smartmirror-daemon ${DESTDIR}${PREFIX}/usr/libexec/lastore-daemon

	mkdir -p ${DESTDIR}${PREFIX}/usr && cp -rf usr ${DESTDIR}${PREFIX}/
	cp -rf etc ${DESTDIR}${PREFIX}/etc

	mkdir -p ${DESTDIR}${PREFIX}/var/lib/lastore/
	cp -rf var/lib/lastore/* ${DESTDIR}${PREFIX}/var/lib/lastore/
	cp -rf lib ${DESTDIR}${PREFIX}/

	mkdir -p ${DESTDIR}${PREFIX}/var/cache/lastore

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

check_code_quality:
	${GoPath} go vet ./src/...
