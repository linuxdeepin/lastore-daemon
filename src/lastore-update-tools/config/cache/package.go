package cache

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	// "github.com/aptly-dev/aptly/deb"
)

var (
	canonicalOrderBinary = []string{
		"Package",
		"Essential",
		"Status",
		"Priority",
		"Section",
		"Installed-Size",
		// "Maintainer",
		// "Original-Maintainer",
		"Architecture",
		"Source",
		"Version",
		"Replaces",
		"Provides",
		"Depends",
		"Pre-Depends",
		"Recommends",
		"Suggests",
		"Conflicts",
		"Breaks",
		"Conffiles",
		"Filename",
		"Size",
		// "MD5Sum",
		// "MD5sum",
		// "SHA1",
		"SHA256",
		// "SHA512",
		// "Description",
	}
	sysRealArch string
)

var softwareFieldMap map[string]int

func init() {
	var binary Software
	sut := reflect.TypeOf(binary)
	softwareFieldMap = map[string]int{}
	for i := 0; i < sut.NumField(); i++ {
		v := sut.Field(i).Tag.Get("publish")
		switch v {
		case "-":
			continue
		case "":
			softwareFieldMap[sut.Field(i).Name] = i
		default:
			names := strings.Split(v, ",")
			for name := range names {
				softwareFieldMap[names[name]] = i
			}
		}
	}

	cmd := exec.Command("/usr/bin/dpkg", "--print-architecture")

	var output bytes.Buffer
	cmd.Stdout = &output
	err := cmd.Run()
	if err == nil {
		sysRealArch = strings.TrimSpace(output.String())
	}
}

type Software struct {
	Package       string `publish:"Package"`        // 软件包名
	Essential     string `publish:"Essential"`      // 该软件包是否是必须的
	Status        string `publish:"Status"`         //
	Priority      string `publish:"Priority"`       // 软件包优先级
	Section       string `publish:"Section"`        // 软件包类别
	InstalledSize string `publish:"Installed-Size"` // 安装后大小
	// Maintainer         string            `publish:"Maintainer"`          // 软件包维护者
	// OriginalMaintainer string            `publish:"Original-Maintainer"` // 原始维护人员
	Architecture string `publish:"Architecture"` // 软件包架构
	Source       string `publish:"Source"`       // 软件包源码名称
	Version      string `publish:"Version"`      // 软件包版本
	Replaces     string `publish:"Replaces"`     // 可替代的软件包
	Provides     string `publish:"Provides"`     // 提供者
	Depends      string `publish:"Depends"`      // 软件包依赖
	PreDepends   string `publish:"Pre-Depends"`  // 软件安装前必须安装
	Recommends   string `publish:"Recommends"`   // 推荐安装软件包和库文件
	Suggests     string `publish:"Suggests"`     // 建议安装
	Conflicts    string `publish:"Conflicts"`    // 存在冲突的软件包
	Breaks       string `publish:"Breaks"`       //
	Conffiles    string `publish:"Conffiles"`    // 存在冲突的软件
	Filename     string `publish:"Filename"`     // 文件名称
	Size         string `publish:"Size"`         // 文件大小
	// MD5Sum       string            `publish:"MD5Sum"`       // md5
	// SHA1         string            `publish:"SHA1"`         // sha1
	SHA256 string `publish:"SHA256"` // sha256
	// SHA512      string            `publish:"SHA512"`      // sha512
	// Description string            `publish:"Description"` // 描述
	// Homepage    string            `publish:"Homepage"`    // 主页
	// OtherStanza map[string]string `publish:"-" gorm:"-"`  // 剩余信息
	Components string `publish:"-"` // 组件(main/non-free...)
}

func (sw Software) String() string {
	bts, err := sw.Encode()
	if err != nil {
		return err.Error()
	}
	return string(bts)
}

func (sw *Software) Encode() ([]byte, error) {
	buff := bytes.NewBuffer(nil)
	value := reflect.ValueOf(sw).Elem()
	var err error

	for _, key := range canonicalOrderBinary {
		if index, has := softwareFieldMap[key]; !has {
			continue
		} else {
			field := value.Field(index)
			str := field.String()
			if str == "" {
				continue
			}
			if err = Encode(buff, key, field.String()); err != nil {
				return nil, err
			}
		}
	}
	// err = EncodeMap(buff, sw.OtherStanza)
	return buff.Bytes(), err
}

// Stanza 从控制文件节中获取数据
func (sw *Software) Stanza(stanza map[string]string) {
	value := reflect.ValueOf(sw).Elem()
	for k, v := range stanza {
		if index, has := softwareFieldMap[k]; has {
			value.Field(index).SetString(v)
		}
		// else {
		// 	if sw.OtherStanza == nil {
		// 		sw.OtherStanza = map[string]string{}
		// 	}
		// 	sw.OtherStanza[k] = v
		// }
	}
}

// UniqueName 全局唯一软件名称
func (sw *Software) UniqueName() string {
	return fmt.Sprintf("%s %s %s", sw.Package, sw.Version, sw.Architecture)
}

func (sw *Software) DependsList() ([][2]string, error) {
	if sw.Depends = strings.TrimSpace(sw.Depends); sw.Depends == "" {
		return nil, nil
	}
	dps := strings.Split(sw.Depends, ",")
	result := make([][2]string, 0, len(dps))
	for i := range dps {
		startIndex := strings.Index(dps[i], "(")
		endIndex := strings.LastIndex(dps[i], ")")
		if startIndex == -1 || endIndex == -1 {
			return nil, errors.New("错误的版本依赖关系")
		}
		result = append(result, [2]string{
			strings.TrimSpace(dps[i][:startIndex]),
			strings.TrimSpace(dps[i][startIndex+1 : endIndex]),
		})
	}
	return result, nil
}

// MergePackagesSoftware 合并软件集合(不同仓库源或同源下重复信息)
func MergePackagesSoftware(instrict bool, sws ...[]*Software) ([]*Software, error) {
	r, _, e := mergePackagesSoftware(instrict, sws...)
	return r, e
}

// MergePackagesSoftwareWarm 合并软件集合(不同仓库源或同源下重复信息通过数据返回，不终止)
func MergePackagesSoftwareWarm(sws ...[]*Software) ([]*Software, map[string][]*Software, error) {
	return mergePackagesSoftware(false, sws...)
}

func mergePackagesSoftware(instrict bool, sws ...[]*Software) ([]*Software, map[string][]*Software, error) {
	temp := map[string]*Software{}
	same := map[string][]*Software{}
	for _, item := range sws {
		for _, subitem := range item {
			name := subitem.UniqueName()
			if v, has := temp[name]; has {
				if instrict {
					if v.SHA256 != "" && subitem.SHA256 != "" && subitem.SHA256 != v.SHA256 {
						return nil, nil, fmt.Errorf("%s:同架构、同名称、同版本软件摘要信息不一致", name)
					}
					// if (v.MD5Sum != "" && subitem.MD5Sum != "" && subitem.MD5Sum != v.MD5Sum) ||
					// 	(v.SHA1 != "" && subitem.SHA1 != "" && subitem.SHA1 != v.SHA1) ||
					// 	(v.SHA256 != "" && subitem.SHA256 != "" && subitem.SHA256 != v.SHA256) ||
					// 	(v.SHA512 != "" && subitem.SHA512 != "" && subitem.SHA512 != v.SHA512) {
					// 	return nil, nil, fmt.Errorf("%s:同架构、同名称、同版本软件摘要信息不一致", name)
					// }
				} else {
					same[name] = append(same[name], subitem)
				}
			} else {
				temp[name] = subitem
			}
		}
	}

	result := make([]*Software, 0)
	for _, v := range temp {
		result = append(result, v)
	}
	return result, same, nil
}

func makeStanzaAppInfo(stanza Stanza) (bool, AppInfo) {
	appinfo := AppInfo{}
	if d, ok := stanza["Package"]; !ok {
		return false, appinfo
	} else {
		appinfo.Name = d
	}
	if d, ok := stanza["Version"]; !ok {
		return false, appinfo
	} else {
		appinfo.Version = d
	}
	if d, ok := stanza["Architecture"]; !ok {
		return false, appinfo
	} else {
		appinfo.Arch = d
	}
	if d, ok := stanza["Filename"]; !ok {
		return false, appinfo
	} else {
		appinfo.Filename = path.Base(d)
	}
	if d, ok := stanza["SHA256"]; !ok {
		return false, appinfo
	} else {
		appinfo.HashSha256 = d
	}

	if d, ok := stanza["Size"]; ok {
		if debSize, err := strconv.Atoi(d); err == nil {
			appinfo.DebSize = debSize
		}
	} else {
		appinfo.DebSize = -1
	}

	if d, ok := stanza["Installed-Size"]; ok {
		if debSize, err := strconv.Atoi(d); err == nil {
			appinfo.InstalledSize = debSize
		}
	} else {
		appinfo.InstalledSize = -1
	}

	return true, appinfo
}
func DecodeStanzaByCacheInfo(reader io.Reader, transform func(map[string]string) (interface{}, error), t *CacheInfo) ([]interface{}, error) {
	result := make([]interface{}, 0)
	return result, nil
}

func DecodeStanzaByList(reader io.Reader, transform func(map[string]string) (interface{}, error), t []string) ([]interface{}, error) {
	result := make([]interface{}, 0)
	sreader := NewControlFileReader(reader, false, false)

	for {
		stanza, err := sreader.ReadStanza()
		if err != nil {
			return nil, err
		}
		if stanza == nil {
			break
		}
		if transform != nil {
			item, err := transform(stanza)
			if err != nil {
				return nil, err
			}
			for _, pkg := range t {
				if d, ok := stanza["Package"]; !ok {
					break
				} else {
					if d == pkg {
						result = append(result, item)
					}

				}
			}

		}
	}
	return result, nil
}

func DecodeStanza(reader io.Reader, transform func(map[string]string) (interface{}, error)) ([]interface{}, error) {
	result := make([]interface{}, 0)
	sreader := NewControlFileReader(reader, false, false)

	for {
		stanza, err := sreader.ReadStanza()
		if err != nil {
			return nil, err
		}
		if stanza == nil {
			break
		}
		if transform != nil {
			item, err := transform(stanza)
			if err != nil {
				return nil, err
			}
			result = append(result, item)
		}
	}
	return result, nil
}

// DiffPackagesSoftware 计算两个软件仓库集合中的软件差异
func DiffPackagesSoftware(coll, colr []*Software) (ownl []*Software, ownr []*Software) {
	sort.Slice(coll, func(i, j int) bool {
		return coll[i].UniqueName() < coll[j].UniqueName()
	})

	sort.Slice(colr, func(i, j int) bool {
		return colr[i].UniqueName() < colr[j].UniqueName()
	})

	ownl = make([]*Software, 0)
	ownr = make([]*Software, 0)

	lengthA := len(coll)
	lengthB := len(colr)
	i, j := 0, 0

	for {
		if i >= lengthA {
			ownr = append(ownr, colr[j:]...)
			break
		}
		if j >= lengthB {
			ownl = append(ownl, coll[i:]...)
			break
		}

		switch {
		case coll[i].UniqueName() == colr[j].UniqueName():
			i++
			j++
			continue
		case coll[i].UniqueName() > colr[j].UniqueName():
			ownr = append(ownr, colr[j])
			j++
		default:
			ownl = append(ownl, coll[i])
			i++
		}
	}
	return
}

/****************************** 文本信息编码 *******************************************************************/
var checkKeyReg = regexp.MustCompile("^[ -0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz]{0,}$")

func checkKey(str string) bool {
	return checkKeyReg.MatchString(str)
}

func checkValue(str string) bool {
	rs := []byte(str)
	for i := range rs {
		if !(rs[i] > 127 || (rs[i] > 31 && rs[i] < 127) || rs[i] == '\n' || rs[i] == '\t') {
			return false
		}
	}
	return true
}

func Encode(writer io.Writer, k, v string) error {
	if !checkKey(k) {
		return fmt.Errorf("错误的Key值:%s", k)
	}
	if !checkValue(v) {
		return errors.New("错误的Value值")
	}

	strBuild := strings.Builder{}
	strBuild.WriteString(k)
	strBuild.WriteString(": ")

	lines := strings.Split(v, "\n")
	length := len(lines)
	for i := 0; i < length; i++ {
		strBuild.WriteByte(' ')
		strBuild.WriteString(lines[i])
		strBuild.WriteString("\n")
	}
	_, err := writer.Write([]byte(strBuild.String()))
	return err
}

func EncodeMap(writer io.Writer, kv Stanza) error {
	for k, v := range kv {
		if !checkKey(k) {
			return fmt.Errorf("错误的Key值:%s", k)
		}
		if !checkValue(v) {
			return fmt.Errorf("错误的Value值,对应Key值:%s", k)
		}
	}

	build := strings.Builder{}
	for k, v := range kv {
		if v == "" {
			continue
		}
		build.WriteString(k)
		build.WriteString(": ")

		lines := strings.Split(v, "\n")
		length := len(lines)
		for i := 0; i < length-1; i++ {
			build.WriteString(lines[i])
			build.WriteString("\n ")
		}
		build.WriteString(lines[length-1])
		build.WriteString("\n")
	}
	_, err := writer.Write([]byte(build.String()))
	return err
}

type Stanza map[string]string

func Decode(reader io.Reader) (Stanza, error) {
	result := Stanza{}
	scan := bufio.NewScanner(reader)
	key := ""
	value := ""
	itemSize := 2
	for scan.Scan() {
		line := scan.Text()
		if line == "" {
			if key != "" {
				result[key] = value
			}
			if len(result) > 0 {
				return result, nil
			}
			continue
		}
		if line[0] == ' ' || line[0] == '\t' {
			value = value + "\n" + line[1:]
		} else {
			if key != "" { // 用于判断直接发送HEADER头后就结束的消息
				result[key] = value
			}
			subs := strings.SplitN(line, ": ", itemSize)
			if len(subs) != itemSize {
				return nil, errors.New("")
			}
			key = subs[0]
			value = subs[1]
		}
	}

	return result, nil
}

const MaxFieldSize = 2 * 1024 * 1024

// Parsing errors
var (
	ErrMalformedStanza = errors.New("malformed stanza syntax")
)

func isMultilineField(field string, isRelease bool) bool {
	switch field {
	// file without a section
	case "":
		return true
	case "Description":
		return true
	case "Files":
		return true
	case "Changes":
		return true
	case "Checksums-Sha1":
		return true
	case "Checksums-Sha256":
		return true
	case "Checksums-Sha512":
		return true
	case "Package-List":
		return true
	case "MD5Sum":
		return isRelease
	case "SHA1":
		return isRelease
	case "SHA256":
		return isRelease
	case "SHA512":
		return isRelease
	}
	return false
}

func canonicalCase(field string) string {
	upper := strings.ToUpper(field)
	switch upper {
	case "SHA1", "SHA256", "SHA512":
		return upper
	case "MD5SUM":
		return "MD5Sum"
	case "NOTAUTOMATIC":
		return "NotAutomatic"
	case "BUTAUTOMATICUPGRADES":
		return "ButAutomaticUpgrades"
	}

	startOfWord := true

	return strings.Map(func(r rune) rune {
		if startOfWord {
			startOfWord = false
			return unicode.ToUpper(r)
		}

		if r == '-' {
			startOfWord = true
		}

		return unicode.ToLower(r)
	}, field)
}

// ControlFileReader implements reading of control files stanza by stanza
type ControlFileReader struct {
	scanner     *bufio.Scanner
	isRelease   bool
	isInstaller bool
}

// NewControlFileReader creates ControlFileReader, it wraps with buffering
func NewControlFileReader(r io.Reader, isRelease, isInstaller bool) *ControlFileReader {
	scnr := bufio.NewScanner(bufio.NewReaderSize(r, 32768))
	scnr.Buffer(nil, MaxFieldSize)

	return &ControlFileReader{
		scanner:     scnr,
		isRelease:   isRelease,
		isInstaller: isInstaller,
	}
}

// ReadStanza reeads one stanza from control file
func (c *ControlFileReader) ReadStanza() (Stanza, error) {
	stanza := make(Stanza, 32)
	lastField := ""
	lastFieldMultiline := c.isInstaller

	for c.scanner.Scan() {
		line := c.scanner.Text()

		// Current stanza ends with empty line
		if line == "" {
			if len(stanza) > 0 {
				return stanza, nil
			}
			continue
		}

		if line[0] == ' ' || line[0] == '\t' || c.isInstaller {
			continue
			// if lastFieldMultiline {
			// 	stanza[lastField] += line + "\n"
			// } else {
			// 	stanza[lastField] += " " + strings.TrimSpace(line)
			// }
		} else {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) != 2 {
				return nil, ErrMalformedStanza
			}
			lastField = canonicalCase(parts[0])
			lastFieldMultiline = isMultilineField(lastField, c.isRelease)
			if lastFieldMultiline {
				stanza[lastField] = parts[1]
				if parts[1] != "" {
					stanza[lastField] += "\n"
				}
			} else {
				stanza[lastField] = strings.TrimSpace(parts[1])
			}
		}
	}
	if err := c.scanner.Err(); err != nil {
		return nil, err
	}
	if len(stanza) > 0 {
		return stanza, nil
	}
	return nil, nil
}
