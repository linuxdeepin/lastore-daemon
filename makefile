PBUILDER_PKG = pbuilder-satisfydepends-dummy

GOPKG_PREFIX = github.com/linuxdeepin/lastore-daemon

GOPATH_DIR = gopath

pwd := ${shell pwd}
GoPath := GOPATH=${pwd}:${pwd}/vendor:${CURDIR}/${GOPATH_DIR}:${GOPATH}

GOBUILD = go build
GOTEST = go test -v
export GO111MODULE=off

all:  build

TEST = \
	${GOPKG_PREFIX}/src/internal/system \
	${GOPKG_PREFIX}/src/internal/system/apt \
	${GOPKG_PREFIX}/src/internal/utils \
	${GOPKG_PREFIX}/src/internal/querydesktop \
	${GOPKG_PREFIX}/src/lastore-daemon \
	${GOPKG_PREFIX}/src/lastore-smartmirror \
	${GOPKG_PREFIX}/src/lastore-tools \
	${GOPKG_PREFIX}/src/lastore-smartmirror-daemon \
	./src/lastore-update-tools/...

prepare:
	@mkdir -p out/bin
	@mkdir -p ${GOPATH_DIR}/src/$(dir ${GOPKG_PREFIX});
	@ln -snf ../../../.. ${GOPATH_DIR}/src/${GOPKG_PREFIX};

bin/lastore-agent:src/lastore-agent/*.c
	@mkdir -p bin
	gcc ${SECURITY_BUILD_OPTIONS} -W -Wall -D_GNU_SOURCE -o $@ $^ $(shell pkg-config --cflags --libs glib-2.0 libsystemd)

build: prepare bin/lastore-agent
	${GoPath} ${GOBUILD} -o bin/lastore-daemon ${GOBUILD_OPTIONS} ${GOPKG_PREFIX}/src/lastore-daemon
	${GoPath} ${GOBUILD} -o bin/lastore-tools ${GOBUILD_OPTIONS} ${GOPKG_PREFIX}/src/lastore-tools
	${GoPath} ${GOBUILD} -o bin/lastore-smartmirror ${GOBUILD_OPTIONS} ${GOPKG_PREFIX}/src/lastore-smartmirror || echo "build failed, disable smartmirror support "
	${GoPath} ${GOBUILD} -o bin/lastore-smartmirror-daemon ${GOBUILD_OPTIONS} ${GOPKG_PREFIX}/src/lastore-smartmirror-daemon || echo "build failed, disable smartmirror support "
	${GoPath} ${GOBUILD} -o bin/lastore-apt-clean ${GOBUILD_OPTIONS} ${GOPKG_PREFIX}/src/lastore-apt-clean

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
	${GoPath} ${GOTEST} ${TEST}

test-coverage:
	env ${GoPath} go test -cover -v ./src/... | awk '$$1 ~ "^(ok|\\?)" {print $$2","$$5}' | sed "s:${CURDIR}::g" | sed 's/files\]/0\.0%/g' > coverage.csv


print_gopath:
	GOPATH="${pwd}:${pwd}/vendor:${GOPATH}"

install: gen_mo
	mkdir -p ${DESTDIR}${PREFIX}/usr/bin && cp bin/lastore-apt-clean ${DESTDIR}${PREFIX}/usr/bin/
	cp bin/lastore-tools ${DESTDIR}${PREFIX}/usr/bin/
	cp bin/lastore-smartmirror ${DESTDIR}${PREFIX}/usr/bin/
	cp bin/lastore-agent ${DESTDIR}${PREFIX}/usr/bin/
	mkdir -p ${DESTDIR}${PREFIX}/usr/libexec/lastore-daemon && cp bin/lastore-daemon ${DESTDIR}${PREFIX}/usr/libexec/lastore-daemon
	cp bin/lastore-smartmirror-daemon ${DESTDIR}${PREFIX}/usr/libexec/lastore-daemon

	mkdir -p ${DESTDIR}${PREFIX}/usr && cp -rf usr ${DESTDIR}${PREFIX}/
	cp -rf etc ${DESTDIR}${PREFIX}/

	mkdir -p ${DESTDIR}${PREFIX}/var/lib/lastore/
	cp -rf var/lib/lastore/* ${DESTDIR}${PREFIX}/var/lib/lastore/
	cp -rf lib ${DESTDIR}${PREFIX}/

	mkdir -p ${DESTDIR}${PREFIX}/var/cache/lastore

	mkdir -p ${DESTDIR}${PREFIX}/var/lib/lastore/check/
	cp -rf configs/config.yaml ${DESTDIR}${PREFIX}/var/lib/lastore/check/config.yaml
	cp -rf configs/caches.yaml ${DESTDIR}${PREFIX}/var/lib/lastore/check/caches.yaml

update_pot:
	deepin-update-pot locale/locale_config.ini

gen_mo:
	deepin-generate-mo locale/locale_config.ini
	mkdir -p ${DESTDIR}${PREFIX}/usr/share/locale/
	cp -rf locale/mo/* ${DESTDIR}${PREFIX}/usr/share/locale/

	deepin-generate-mo locale_categories/locale_config.ini
	cp -rf locale_categories/mo/* ${DESTDIR}${PREFIX}/usr/share/locale/

gen-xml:
	qdbus --system org.deepin.dde.Lastore1 /org/deepin/dde/Lastore1 org.freedesktop.DBus.Introspectable.Introspect > usr/share/dbus-1/interfaces/org.deepin.dde.Lastore1.xml
	qdbus --system org.deepin.dde.Lastore1 /org/deepin/dde/Lastore1/Job1 org.freedesktop.DBus.Introspectable.Introspect > usr/share/dbus-1/interfaces/org.deepin.dde.Lastore1.Job.xml

build-deb:
	yes | debuild -us -uc

clean:
	rm -rf bin
	rm -rf pkg
	rm -rf vendor/pkg
	rm -rf vendor/bin
	rm -rf gopath

check_code_quality:
	${GoPath} go vet ./src/...
