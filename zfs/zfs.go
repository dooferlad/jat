package zfs

import (
	"bufio"
	"fmt"
	"reflect"
	"strings"

	"github.com/dooferlad/here"

	"github.com/dooferlad/jat/shell"
)

type Listing struct {
	Name       string `zfs:"key"`
	Used       string
	Avail      string
	Refer      string
	Mountpoint string
}

func unwrap(wrapped []byte, v interface{}) error {
	pointerTargetType := reflect.TypeOf(v)
	targetValue := reflect.ValueOf(v)
	if pointerTargetType.Kind() != reflect.Ptr {
		return fmt.Errorf("unwrap expects a map[string]interface{}, found %v", pointerTargetType.Kind())
	}
	targetType := pointerTargetType.Elem()
	if targetType.Kind() != reflect.Map {
		return fmt.Errorf("unwrap expects a map[string]interface{}, found %v", targetType.Kind())
	}

	targetStructType := targetType.Elem()
	keyType := targetType.Key()

	if keyType.Kind() != reflect.String {
		return fmt.Errorf("unwrap expects a map[string]interface{}, found %v", targetType.Kind())
	}

	scanner := bufio.NewScanner(strings.NewReader(string(wrapped)))
	var keyValue string

	for scanner.Scan() {
		parts := strings.Split(scanner.Text(), "\t")
		newElement := reflect.New(targetStructType)

		for i, part := range parts {
			newElement.Elem().Field(i).SetString(part)

			if name, ok := targetStructType.Field(i).Tag.Lookup("zfs"); ok {
				if name == "key" {
					keyValue = part
				}
			}
		}

		pv := targetValue.Elem()
		if pv.IsNil() {
			pv.Set(reflect.MakeMap(targetType))
		}
		pv.SetMapIndex(reflect.ValueOf(keyValue), newElement.Elem())
	}

	return nil
}

func List() (*map[string]Listing, error) {
	out, err := shell.Capture("sudo", "zfs", "list", "-H")
	if err != nil {
		return nil, err
	}

	var listing *map[string]Listing
	listing = new(map[string]Listing)
	err = unwrap(out, listing)

	here.Is(listing)

	return listing, err
}
