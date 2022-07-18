PBUILDER_PKG = pbuilder-satisfydepends-dummy

pwd := ${shell pwd}
GoPath := GOPATH=${pwd}:${pwd}/vendor:${GOPATH}

GOBUILD = go build
GOTEST = go test -v
export GO111MODULE=off

all:  build

build:
	${GoPath} ${GOBUILD} -o bin/lastore-daemon ${GOBUILD_OPTIONS} lastore-daemon
	${GoPath} ${GOBUILD} -o bin/lastore-tools ${GOBUILD_OPTIONS} lastore-tools
	${GoPath} ${GOBUILD} -o bin/lastore-smartmirror ${GOBUILD_OPTIONS} lastore-smartmirror || echo "build failed, disable smartmirror support "
	${GoPath} ${GOBUILD} -o bin/lastore-smartmirror-daemon ${GOBUILD_OPTIONS} lastore-smartmirror-daemon || echo "build failed, disable smartmirror support "
	${GoPath} ${GOBUILD} -o bin/lastore-apt-clean ${GOBUILD_OPTIONS} lastore-apt-clean

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
	qdbus --system org.deepin.lastore1 /org/deepin/lastore1 org.freedesktop.DBus.Introspectable.Introspect > usr/share/dbus-1/interfaces/org.deepin.lastore1.xml
	qdbus --system org.deepin.lastore1 /org/deepin/lastore1/Job1 org.freedesktop.DBus.Introspectable.Introspect > usr/share/dbus-1/interfaces/org.deepin.lastore1.Job.xml

build-deb:
	yes | debuild -us -uc

clean:
	rm -rf bin
	rm -rf pkg
	rm -rf vendor/pkg
	rm -rf vendor/bin

check_code_quality:
	${GoPath} go vet ./src/...
