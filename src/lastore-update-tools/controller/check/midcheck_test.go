package check

import (
	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/config/cache"
	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/pkg/utils/ecode"
	"testing"
)

func TestCheckPkgDependency(t *testing.T) {
	testCases := []struct {
		name             string
		sysCurrPackage   map[string]*cache.AppTinyInfo
		expectedCode     int64
		expectedErrorMsg string
	}{
		{
			name: "All packages are in check state",
			sysCurrPackage: map[string]*cache.AppTinyInfo{
				"libc-bin":   &cache.AppTinyInfo{Name: "libc-bin", Version: "1", State: "ii"},
				"systemd":    &cache.AppTinyInfo{Name: "systemd", Version: "1", State: "ii"},
				"lightdm":    &cache.AppTinyInfo{Name: "lightdm", Version: "1", State: "ii"},
				"alsa-utils": &cache.AppTinyInfo{Name: "alsa-utils", Version: "1", State: "ii"},
				"uos-test":   &cache.AppTinyInfo{Name: "uos-test", Version: "1", State: "hU"},
			},
			expectedCode:     ecode.CHK_PKG_DEPEND_ERROR,
			expectedErrorMsg: "found package state err: dependency error: dpkg:<nil> dependency error: apt:<nil> dependency error: bluez:<nil> dependency error: pulseaudio:<nil> dependency error: network-manager:<nil> dependency error: libpam-modules:<nil> dependency error: cryptsetup:<nil> dependency error: passwd:<nil> dependency error: policykit-1:<nil> dependency error: libgbm1:<nil> dependency error: xserver-xorg:<nil> dependency error: libwayland-client0:<nil> dependency error: dbus:<nil> dependency error: libgtk-3-0:<nil> dependency error: permission-manager:<nil> dependency error: deepin-elf-verify:<nil> dependency error: dde-dock:<nil> dependency error: dde-launcher:<nil> dependency error: dde-control-center:<nil> dependency error: dde-desktop:<nil> dependency error: dde-file-manager:<nil> dependency error: lastore-daemon:<nil> dependency error: dde-session-shell:<nil> dependency error: startdde:<nil> dependency error: dde-daemon:<nil> dependency error: libdtkcore5:<nil> dependency error: libdtkwidget5:<nil> dependency error: libdtkgui5:<nil> dependency error: deepin-authenticate:<nil> dependency error: dde-polkit-agent:<nil> dependency error: deepin-keyring:<nil> dependency error: deepin-license-activator:<nil>",
		},
		{
			name: "Some packages are not in check state",
			sysCurrPackage: map[string]*cache.AppTinyInfo{
				"libc-bin":   &cache.AppTinyInfo{Name: "libc-bin", Version: "1", State: "hU"},
				"systemd":    &cache.AppTinyInfo{Name: "systemd", Version: "1", State: "ii"},
				"lightdm":    &cache.AppTinyInfo{Name: "lightdm", Version: "1", State: "ii"},
				"alsa-utils": &cache.AppTinyInfo{Name: "alsa-utils", Version: "1", State: "ii"},
			},
			expectedCode:     ecode.CHK_PKG_DEPEND_ERROR,
			expectedErrorMsg: "found package state err: dependency error: libc-bin:&{libc-bin 1 hU} dependency error: dpkg:<nil> dependency error: apt:<nil> dependency error: bluez:<nil> dependency error: pulseaudio:<nil> dependency error: network-manager:<nil> dependency error: libpam-modules:<nil> dependency error: cryptsetup:<nil> dependency error: passwd:<nil> dependency error: policykit-1:<nil> dependency error: libgbm1:<nil> dependency error: xserver-xorg:<nil> dependency error: libwayland-client0:<nil> dependency error: dbus:<nil> dependency error: libgtk-3-0:<nil> dependency error: permission-manager:<nil> dependency error: deepin-elf-verify:<nil> dependency error: dde-dock:<nil> dependency error: dde-launcher:<nil> dependency error: dde-control-center:<nil> dependency error: dde-desktop:<nil> dependency error: dde-file-manager:<nil> dependency error: lastore-daemon:<nil> dependency error: dde-session-shell:<nil> dependency error: startdde:<nil> dependency error: dde-daemon:<nil> dependency error: libdtkcore5:<nil> dependency error: libdtkwidget5:<nil> dependency error: libdtkgui5:<nil> dependency error: deepin-authenticate:<nil> dependency error: dde-polkit-agent:<nil> dependency error: deepin-keyring:<nil> dependency error: deepin-license-activator:<nil>",
		},
		{
			name: "A package is missing",
			sysCurrPackage: map[string]*cache.AppTinyInfo{
				"libc-bin":   &cache.AppTinyInfo{Name: "libc-bin", Version: "1", State: "ii"},
				"systemd":    &cache.AppTinyInfo{Name: "systemd", Version: "1", State: "ii"},
				"alsa-utils": &cache.AppTinyInfo{Name: "alsa-utils", Version: "1", State: "ii"},
			},
			expectedCode:     ecode.CHK_PKG_DEPEND_ERROR,
			expectedErrorMsg: "found package state err: dependency error: lightdm:<nil> dependency error: dpkg:<nil> dependency error: apt:<nil> dependency error: bluez:<nil> dependency error: pulseaudio:<nil> dependency error: network-manager:<nil> dependency error: libpam-modules:<nil> dependency error: cryptsetup:<nil> dependency error: passwd:<nil> dependency error: policykit-1:<nil> dependency error: libgbm1:<nil> dependency error: xserver-xorg:<nil> dependency error: libwayland-client0:<nil> dependency error: dbus:<nil> dependency error: libgtk-3-0:<nil> dependency error: permission-manager:<nil> dependency error: deepin-elf-verify:<nil> dependency error: dde-dock:<nil> dependency error: dde-launcher:<nil> dependency error: dde-control-center:<nil> dependency error: dde-desktop:<nil> dependency error: dde-file-manager:<nil> dependency error: lastore-daemon:<nil> dependency error: dde-session-shell:<nil> dependency error: startdde:<nil> dependency error: dde-daemon:<nil> dependency error: libdtkcore5:<nil> dependency error: libdtkwidget5:<nil> dependency error: libdtkgui5:<nil> dependency error: deepin-authenticate:<nil> dependency error: dde-polkit-agent:<nil> dependency error: deepin-keyring:<nil> dependency error: deepin-license-activator:<nil>",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			code, err := CheckPkgDependency(tc.sysCurrPackage)
			if code != tc.expectedCode {
				t.Errorf("Expected code %d, got %d", tc.expectedCode, code)
			}
			if err != nil && err.Error() != tc.expectedErrorMsg {
				t.Errorf("Expected error message '%s', got '%s'", tc.expectedErrorMsg, err.Error())
			}
		})
	}
}
