package main

import (
	"dbus/org/freedesktop/login1"
	"internal/system"
	"os"
	"path"
	"pkg.deepin.io/lib/dbus"
	"strings"
)

// QueryLang return user lang.
// the rule is document at man gettext(3)
func QueryLang() string {
	return QueryLangs()[0]
}

// QueryLangs return array of user lang, split by ",".
// the rule is document at man gettext(3)
func QueryLangs() []string {
	LC_ALL := os.Getenv("LC_ALL")
	LC_MESSAGE := os.Getenv("LC_MESSAGE")
	LANGUAGE := os.Getenv("LANGUAGE")
	LANG := os.Getenv("LANG")

	if LC_ALL != "C" && LANGUAGE != "" {
		langs := strings.Split(LANGUAGE, ",")
		return langs
	}

	if LC_ALL != "" {
		return []string{LC_ALL}
	}
	if LC_MESSAGE != "" {
		return []string{LC_MESSAGE}
	}
	if LANG != "" {
		return []string{LANG}
	}
	return []string{""}
}

func PackageName(pkgs []string, lang string) string {
	names := make(map[string]struct {
		Id         string            `json:"id"`
		Name       string            `json:"name"`
		NameLocale map[string]string `json:"name_locale"`
	})

	system.DecodeJson(path.Join(system.VarLibDir, "applications.json"), &names)

	var r []string
	for _, id := range pkgs {
		info, ok := names[id]
		if !ok {
			r = append(r, id)
			continue
		}
		name := info.NameLocale[lang]
		if name == "" {
			r = append(r, info.Name)
		}
	}
	return strings.Join(r, " ")
}

func Inhibitor(what, who, why string) (dbus.UnixFD, error) {
	m, err := login1.NewManager("org.freedesktop.login1", "/org/freedesktop/login1")
	if err != nil {
		return -1, err
	}
	defer login1.DestroyManager(m)
	return m.Inhibit(what, who, why, "block")
}

func FileExist(fpath string) bool {
	_, err := os.Stat(fpath)
	return err == nil || os.IsExist(err)
}
