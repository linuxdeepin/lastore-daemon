package main

import (
	"flag"
	"fmt"
	log "github.com/cihub/seelog"
	"internal/system"
	"internal/system/apt"
	"os"
	"path"
	"pkg.deepin.io/lib"
	"pkg.deepin.io/lib/dbus"
	"time"
)

func getLogConfig() string {
	fmtString := `
<seelog type="sync">
	<outputs formatid="all">
		<filter levels="info,debug,warn,error">
		  <file path="/var/log/lastore/last/daemon.log"/>
		  <console />
		</filter>
	</outputs>

	<formats>
	  <format id="all" format="[%Level] [%Date %Time]@%File.%Line %Msg%n"/>
	</formats>
</seelog>`
	return fmtString
}

var baseLogDir = flag.String("log", "/var/log/lastore", "the directory to store logs")

func setupLog() {
	var logDir = path.Join(*baseLogDir, time.Now().Format("2006-1-02 15:04:05"))

	err := os.MkdirAll(logDir, 0755)
	if err != nil {
		panic(fmt.Sprintf("Can't create base Dir %v", err))
	}
	lastDir := path.Join(*baseLogDir, "last")
	os.Remove(lastDir)
	err = os.Symlink(logDir, lastDir)
	if err != nil {
		panic(err)
	}

	system.SetupLogDir(logDir)

	logger, err := log.LoggerFromConfigAsBytes([]byte(getLogConfig()))
	if err != nil {
		panic(err)
	}
	log.ReplaceLogger(logger)
}

func main() {
	flag.Parse()

	setupLog()
	defer log.Flush()

	os.Unsetenv("LC_ALL")
	os.Unsetenv("LANGUAGE")
	os.Unsetenv("LC_MESSAGES")
	os.Unsetenv("LANG")

	if os.Getenv("DBUS_STARTER_BUS_TYPE") != "" {
		os.Setenv("PATH", os.Getenv("PATH")+":/bin:/sbin:/usr/bin:/usr/sbin")
	}
	if !lib.UniqueOnSystem("com.deepin.lastore") {
		log.Info("Can't obtain the com.deepin.lastore")
		return
	}

	b := apt.New()
	m := NewManager(b)

	err := dbus.InstallOnSystem(m)
	if err != nil {
		log.Error("Install manager on system bus :", err)
		return
	}
	log.Info("Started service at system bus")

	err = dbus.InstallOnSystem(m.updater)
	if err != nil {
		log.Error("Start failed:", err)
		return
	}

	dbus.DealWithUnhandledMessage()

	if err := dbus.Wait(); err != nil {
		log.Warn("DBus Error:", err)
	}
}
