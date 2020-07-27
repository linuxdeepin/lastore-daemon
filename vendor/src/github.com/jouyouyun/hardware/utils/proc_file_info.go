package utils

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func ProcGetByKey(filename, delim string, keySet map[string]string,
	fall bool) error {
	fr, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer fr.Close()

	l := len(keySet)
	count := 0
	scanner := bufio.NewScanner(fr)
	for scanner.Scan() {
		if count >= l && !fall {
			break
		}
		text := scanner.Text()
		if len(text) == 0 {
			continue
		}
		items := strings.SplitN(text, delim, 2)
		if len(items) != 2 {
			continue
		}
		key := strings.TrimSpace(items[0])
		_, ok := keySet[key]
		if !ok {
			continue
		}
		keySet[key] = strings.TrimSpace(items[1])
		count++
	}
	if l != count {
		return fmt.Errorf("not found all keys")
	}
	return nil
}
