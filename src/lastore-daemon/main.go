package main

import (
	"flag"
	"internal/system"
	"internal/system/apt"
	"io"
	"log"
	"os"
	"path"
	"pkg.deepin.io/lib"
	"pkg.deepin.io/lib/dbus"
	"time"
)

var baseLogDir = flag.String("log", "/var/log/lastore", "the directory to store logs")

func setupLog() io.WriteCloser {
	var logDir = path.Join(*baseLogDir,
		time.Now().Format("2006-1-02 15:04:05"))
	err := os.MkdirAll(logDir, 0755)
	if err != nil {
		log.Fatal("Can't create base Dir", err)
	}
	lastDir := path.Join(*baseLogDir, "last")
	os.Remove(lastDir)
	err = os.Symlink(logDir, lastDir)
	if err != nil {
		log.Fatal(err)
	}

	system.SetupLogDir(logDir)
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	os.Stdout.WriteString("Log is writing to " + lastDir + "\n")

	w, err := os.Create(path.Join(logDir, "daemon.log"))
	if err != nil {
		log.Fatal("Can't create log dir", err)
	}
	log.SetOutput(w)
	return w
}

func main() {
	flag.Parse()

	w := setupLog()
	defer w.Close()

	os.Unsetenv("LC_ALL")
	os.Unsetenv("LANGUAGE")
	os.Unsetenv("LC_MESSAGES")
	os.Unsetenv("LANG")

	if os.Getenv("DBUS_STARTER_BUS_TYPE") != "" {
		os.Setenv("PATH", os.Getenv("PATH")+":/bin:/sbin:/usr/bin:/usr/sbin")
	}
	if !lib.UniqueOnSystem("com.deepin.lastore") {
		log.Println("Can't obtain the com.deepin.lastore")
		return
	}

	b := apt.New()
	m := NewManager(b)

	err := dbus.InstallOnSystem(m)
	if err != nil {
		log.Println("Install manager on system bus :", err)
		return
	}
	log.Println("Started service at system bus")

	err = dbus.InstallOnSystem(m.updater)
	if err != nil {
		log.Println("Start failed:", err)
		return
	}

	dbus.DealWithUnhandledMessage()

	if err := dbus.Wait(); err != nil {
		log.Println("DBus Error:", err)
	}
}
