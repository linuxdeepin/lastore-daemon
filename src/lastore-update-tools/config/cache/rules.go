package cache

import (
	"fmt"
	"reflect"

	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/pkg/utils/fs"
)

const (
	PreUpdate       = iota // pre
	UpdateCheck            // mid
	PostCheck              // post
	PostCheckFailed        // post failed check
	bottomCheck
)

type CheckRules struct {
	Name    string `json:"Name,omitempty" yaml:"Name,omitempty"` // name
	Type    int8   `json:"Type" yaml:"Type"`                     // type
	Command string `json:"Command" yaml:"Command"`               // Command
	Argv    string `json:"Argv" yaml:"Argv"`                     // argv "--ignore-error"
}

func (ts *CheckRules) Merge(rightRules CheckRules) error {

	rightValueList := reflect.ValueOf(rightRules)
	leftValueList := reflect.ValueOf(ts).Elem()

	for i := 0; i < leftValueList.NumField(); i++ {
		leftField := leftValueList.Field(i)
		rightFiled := rightValueList.FieldByName(leftValueList.Type().Field(i).Name)

		if rightFiled.IsValid() && rightFiled.Interface() != "" && !reflect.DeepEqual(rightFiled, leftField) {
			//fmt.Printf("rightFiled: %+v\n", rightFiled.Interface())
			leftField.Set(rightFiled)
		}
	}

	return nil
}

func (ts *CheckRules) IsEmpty() (bool, error) {

	if len(ts.Command) == 0 {
		return true, fmt.Errorf("command empty")
	}
	if ts.Type >= bottomCheck || ts.Type < PreUpdate {
		return true, fmt.Errorf("type not support")
	}
	if len(ts.Name) == 0 {
		return true, fmt.Errorf("name empty")
	}
	return false, nil

}

func (ts *CheckRules) GetName() (string, error) {
	if flags, _ := ts.IsEmpty(); !flags {
		if ts.Name != "" {
			return ts.Name, nil
		} else {
			switch ts.Type {
			case PreUpdate:
				return "precheck", nil
			case UpdateCheck:
				return "midcheck", nil
			case PostCheck:
				return "postcheck", nil
			case PostCheckFailed:
				return "postcheckfailed", nil
			default:
				return "", fmt.Errorf("not support type")
			}

		}
	} else {
		return "", fmt.Errorf("rules is empty")
	}
}

func (ts *CheckRules) SaveCommand(path string) (string, error) {

	cmdName, err := ts.GetName()
	if err != nil {
		return "", fmt.Errorf("save failed:%v", err)
	}

	cmd := path + "/" + cmdName

	filePath, err := fs.CreateFile(cmd)
	if err != nil {
		return "", fmt.Errorf("create file err:%+v", err)
	}

	defer filePath.Close()
	if _, err := filePath.WriteString(ts.Command); err != nil {
		return "", fmt.Errorf("wirte file err:%v", err)
	}

	return cmd, nil
}
