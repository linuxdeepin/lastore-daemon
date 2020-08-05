package main

import (
	"bufio"
	"bytes"
	"errors"
	"flag"
	"fmt"
	"internal/system"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const maxElapsed = time.Hour * 24 * 6 // 6 days

var (
	binDpkg      string
	binDpkgQuery string
	binDpkgDeb   string
	binAptCache  string
)

func mustGetBin(name string) string {
	file, err := exec.LookPath(name)
	if err != nil {
		log.Fatal(err)
	}
	return file
}

var options struct {
	forceDelete bool
}

func init() {
	flag.BoolVar(&options.forceDelete, "force-delete", false, "force delete deb files")
}

func main() {
	flag.Parse()
	log.SetFlags(log.Lshortfile)
	binDpkg = mustGetBin("dpkg")
	binDpkgQuery = mustGetBin("dpkg-query")
	binDpkgDeb = mustGetBin("dpkg-deb")
	binAptCache = mustGetBin("apt-cache")

	_ = os.Setenv("LC_ALL", "C")

	archivesDir, err := system.GetArchivesDir()
	if err != nil {
		log.Fatal(err)
	}
	log.Println("archives dir:", archivesDir)

	fileInfoList, err := ioutil.ReadDir(archivesDir)
	if err != nil {
		log.Fatal(err)
	}

	for _, fileInfo := range fileInfoList {
		if fileInfo.IsDir() {
			continue
		}

		if filepath.Ext(fileInfo.Name()) != ".deb" {
			continue
		}

		log.Println("> ", fileInfo.Name())
		deletePolicy, err := shouldDelete(archivesDir, fileInfo)
		if err != nil {
			log.Println("shouldDelete Warnning:", err)
		}
		switch deletePolicy {
		case DeleteImmediately:
			deleteDeb(archivesDir, fileInfo.Name())
		case DeleteExpired:
			if options.forceDelete {
				deleteDeb(archivesDir, fileInfo.Name())
			} else {
				debChangeTime := getChangeTime(fileInfo)
				if time.Since(debChangeTime) > maxElapsed {
					deleteDeb(archivesDir, fileInfo.Name())
				} else {
					log.Println("delete later")
				}
			}
		case Keep:
			if options.forceDelete {
				deleteDeb(archivesDir, fileInfo.Name())
			} else {
				log.Println("keep")
			}
		}
	}
}

type DeletePolicy uint

const (
	DeleteExpired = iota
	DeleteImmediately
	Keep
)

func shouldDelete(dir string, fileInfo os.FileInfo) (DeletePolicy, error) {
	debInfo, err := getDebInfo(filepath.Join(dir, fileInfo.Name()))
	if err != nil {
		return DeleteImmediately, err
	}
	log.Printf("%#v\n", debInfo)

	status, version, err := queryStatusVersion(debInfo)
	if err != nil {
		return DeleteExpired, err
	}
	log.Printf("current status: %q, version: %q\n", status, version)
	switch status {
	case statusInstallInstalled:
		if compareVersions(debInfo.version, "gt", version) {
			log.Println("deb version great then installed version")
			candidateVersion, err := getCandidateVersion(debInfo)
			if err != nil {
				return DeleteExpired, err
			}

			log.Println("candidate version:", candidateVersion)
			if candidateVersion != debInfo.version {
				log.Println("not the candiate version")
				return DeleteImmediately, nil
			}
			return Keep, nil
		}

		return DeleteImmediately, nil

	case statusRemoveConfigFiles, statusUnknownNotInstalled:
		return DeleteImmediately, nil
	default:
		return DeleteExpired, nil
	}

}

type DebInfo struct {
	name    string
	version string
	arch    string
}

func getControlField(line []byte, key []byte) (string, error) {
	if bytes.HasPrefix(line, key) {
		return string(line[len(key):]), nil
	}
	return "", fmt.Errorf("failed to get control field %s", key[:len(key)-2])
}

func getDebInfo(filename string) (*DebInfo, error) {
	const (
		fieldPkg  = "Package"
		fieldVer  = "Version"
		fieldArch = "Architecture"
		sep       = ": "
	)

	output, err := exec.Command(binDpkgDeb, "-f", "--", filename,
		fieldPkg, fieldVer, fieldArch).Output()
	if err != nil {
		return nil, err
	}
	lines := bytes.Split(output, []byte{'\n'})
	if len(lines) < 3 {
		return nil, errors.New("getDebInfo len(lines) < 3")
	}

	name, err := getControlField(lines[0], []byte(fieldPkg+sep))
	if err != nil {
		return nil, err
	}
	version, err := getControlField(lines[1], []byte(fieldVer+sep))
	if err != nil {
		return nil, err
	}
	arch, err := getControlField(lines[2], []byte(fieldArch+sep))
	if err != nil {
		return nil, err
	}
	return &DebInfo{
		name:    name,
		version: version,
		arch:    arch,
	}, nil
}

const (
	statusNotFound            = "@not-found"
	statusInstallInstalled    = "ii"
	statusUnknownNotInstalled = "un"
	statusRemoveConfigFiles   = "rc"
)

func queryStatusVersion(info *DebInfo) (status, version string, err error) {
	pkg := info.name + ":" + info.arch
	output, err := exec.Command(binDpkgQuery, "-f", "${db:Status-Abbrev}\n${Version}",
		"-W", "--", pkg).CombinedOutput()

	if err != nil {
		if bytes.Contains(output, []byte("no packages found")) {
			return statusNotFound, "", nil
		}
		return
	}

	lines := bytes.Split(output, []byte{'\n'})
	if len(lines) < 2 {
		err = errors.New("queryStatusVersion: len(lines) < 2")
		return
	}

	status = string(bytes.TrimSpace(lines[0]))
	version = string(bytes.TrimSpace(lines[1]))
	return
}

func getCandidateVersion(info *DebInfo) (string, error) {
	pkg := info.name + ":" + info.arch
	output, err := exec.Command(binAptCache, "policy", "--", pkg).Output()
	if err != nil {
		return "", err
	}
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		const candidate = "Candidate:"
		if strings.HasPrefix(line, candidate) {
			return strings.TrimSpace(line[len(candidate):]), nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", errors.New("not found candidate")
}

func compareVersions(ver1, op, ver2 string) bool {
	err := exec.Command(binDpkg, "--compare-versions", "--", ver1, op, ver2).Run()
	return err == nil
}

// getChangeTime get time when file status was last changed.
func getChangeTime(fileInfo os.FileInfo) time.Time {
	stat := fileInfo.Sys().(*syscall.Stat_t)
	return time.Unix(int64(stat.Ctim.Sec), int64(stat.Ctim.Nsec))
}

func deleteDeb(dir, name string) {
	log.Println("delete deb", name)
	err := os.Remove(filepath.Join(dir, name))
	if err != nil {
		log.Printf("deleteDeb error: %v\n", err)
	}
}
