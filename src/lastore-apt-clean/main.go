package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
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
	archivesDir  string
	binDpkg      string
	binDpkgQuery string
	binDpkgDeb   string
	binAptCache  string
	binAptConfig string
)

func mustGetBin(name string) string {
	file, err := exec.LookPath(name)
	if err != nil {
		log.Fatal(err)
	}
	return file
}

func main() {
	log.SetFlags(log.Lshortfile)
	binDpkg = mustGetBin("dpkg")
	binDpkgQuery = mustGetBin("dpkg-query")
	binDpkgDeb = mustGetBin("dpkg-deb")
	binAptCache = mustGetBin("apt-cache")
	binAptConfig = mustGetBin("apt-config")

	os.Setenv("LC_ALL", "C")

	var err error
	archivesDir, err = getArchivesDir()
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
		del, err := shouldDelete(archivesDir, fileInfo)
		if err != nil {
			log.Println("shouldDelete error:", err)
			continue
		}
		if del {
			deleteDeb(fileInfo.Name())
		}

	}
}

/*
$ apt-config --format '%f=%v%n' dump  Dir
Dir=/
Dir::Cache=var/cache/apt
Dir::Cache::archives=archives/
Dir::Cache::srcpkgcache=srcpkgcache.bin
Dir::Cache::pkgcache=pkgcache.bin
*/
func getArchivesDir() (string, error) {
	output, err := exec.Command(binAptConfig, "--format", "%f=%v%n", "dump", "Dir").Output()
	if err != nil {
		return "", err
	}
	lines := strings.Split(string(output), "\n")
	tempMap := make(map[string]string)
	fieldsCount := 0
loop:
	for _, line := range lines {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			switch parts[0] {
			case "Dir", "Dir::Cache", "Dir::Cache::archives":
				tempMap[parts[0]] = parts[1]
				fieldsCount++
				if fieldsCount == 3 {
					break loop
				}
			}
		}
	}
	dir := tempMap["Dir"]
	if dir == "" {
		return "", errors.New("apt-config Dir is empty")
	}

	dirCache := tempMap["Dir::Cache"]
	if dirCache == "" {
		return "", errors.New("apt-config Dir::Cache is empty")
	}
	dirCacheArchives := tempMap["Dir::Cache::archives"]
	if dirCacheArchives == "" {
		return "", errors.New("apt-config Dir::Cache::Archives is empty")
	}

	return filepath.Join(dir, dirCache, dirCacheArchives), nil
}

func shouldDelete(dir string, fileInfo os.FileInfo) (bool, error) {
	debInfo, err := getDebInfo(filepath.Join(dir, fileInfo.Name()))
	if err != nil {
		return false, err
	}
	log.Printf("%#v\n", debInfo)

	installedVersion, _ := getInstalledVersion(debInfo)

	if installedVersion != "" {
		log.Println("installed version:", installedVersion)

		if compareVersions(debInfo.version, "gt", installedVersion) {
			log.Println("deb version great then installed version")
			candidateVersion, err := getCandidateVersion(debInfo)
			if err != nil {
				return false, err
			}

			log.Println("candidate version:", candidateVersion)
			if candidateVersion != debInfo.version {
				log.Println("not the candiate version")
				return true, nil
			}
			return false, nil
		} else {
			return true, nil
		}

	} else {
		log.Println("package not installed")
		// removed or newly added
		debChangeTime := getChangeTime(fileInfo)
		elapsed := time.Since(debChangeTime)
		return elapsed > maxElapsed, nil
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

	output, err := exec.Command(binDpkgDeb, "-f", filename,
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

func getInstalledVersion(info *DebInfo) (string, error) {
	pkg := info.name + ":" + info.arch
	output, err := exec.Command(binDpkgQuery, "-f", "${Version}", "-W", pkg).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
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
	err := exec.Command(binDpkg, "--compare-versions", ver1, op, ver2).Run()
	return err == nil
}

// getChangeTime get time when file status was last changed.
func getChangeTime(fileInfo os.FileInfo) time.Time {
	stat := fileInfo.Sys().(*syscall.Stat_t)
	return time.Unix(int64(stat.Ctim.Sec), int64(stat.Ctim.Nsec))
}

func deleteDeb(name string) {
	log.Println("delete deb", name)
	err := os.Remove(filepath.Join(archivesDir, name))
	if err != nil {
		log.Printf("deleteDeb error: %v\n", err)
	}
}
