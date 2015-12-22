package main

import "dbus/org/freedesktop/accounts"
import "internal/system"
import "fixme/pkg_recommend"
import "fmt"
import "pkg.deepin.io/lib/dbus"

// QueryLangByUID query Language from org.freedesktop.Accounts
func QueryLangByUID(uid int64) (string, error) {
	ac, err := accounts.NewAccounts("org.freedesktop.Accounts", "/org/freedesktop/Accounts")
	if err != nil {
		return "", err
	}
	defer accounts.DestroyAccounts(ac)
	upath, err := ac.FindUserById(uid)
	if err != nil {
		return "", err
	}

	u, err := accounts.NewUser("org.freedesktop.Accounts", upath)
	if err != nil {
		return "", err
	}
	defer accounts.DestroyUser(u)
	lang := u.Language.Get()
	if lang == "" {
		return "", system.NotFoundError
	}
	return lang, nil
}

func QueryEnhancedLocalePackages(checker func(string) bool, lang string, pkgs ...string) []string {
	set := make(map[string]struct{})
	for _, pkg := range pkgs {
		fmt.Println("Query....", pkg, pkg_recommend.GetEnhancedLocalePackages("zh_CN.UTF-8", pkg))
		for _, localePkg := range pkg_recommend.GetEnhancedLocalePackages(lang, pkg) {
			set[localePkg] = struct{}{}
		}
	}

	var r []string
	for pkg := range set {
		if checker(pkg) {
			r = append(r, pkg)
		}
	}
	return r
}

// Don't directly use this API. It will be migration to com.deepin.Accounts
func (m *Manager) RecordLocaleInfo(msg dbus.DMessage, locale string) error {
	uid := msg.GetSenderUID()
	if locale == "" {
		return system.NotFoundError
	}
	m.cachedLocale[uint64(uid)] = locale
	return nil
}
