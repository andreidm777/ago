package customjson

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
)

func CustomMarshal(v interface{}) ([]byte, error) {
	what := reflect.TypeOf(v)
	val := reflect.ValueOf(v)
	switch what.Kind() {
	case reflect.Array:
		fallthrough
	case reflect.Slice:
		buffer := bytes.NewBufferString("[")
		sarr := val.Len()
		for i := 0; i < sarr; i++ {
			tmp, err := CustomMarshal(val.Index(i).Interface())
			if err != nil {
				return nil, err
			}
			buffer.WriteString(fmt.Sprintf("%s", tmp))
			if sarr > 1 && i < sarr-1 {
				buffer.WriteString(",")
			}
		}
		buffer.WriteString("]")
		return buffer.Bytes(), nil
	case reflect.Struct:
		fallthrough
	case reflect.Chan:
		return nil, errors.New("dont support type")
	case reflect.Map:
        imap := val.Len()
		buffer := bytes.NewBufferString("{")
		for i, k := range val.MapKeys() {
			tmp_key, err := CustomMarshal(k.Interface())
			if err != nil {
				return nil, err
			}
			tmp_val, err := CustomMarshal(val.MapIndex(k).Interface())
			if err != nil {
				return nil, err
			}
			buffer.WriteString(fmt.Sprintf("%s: %s", tmp_key, tmp_val))
            if i < imap - 1 {
                buffer.WriteString(",")
            }
		}
		buffer.WriteString("}")
		return buffer.Bytes(), nil
	default:
		return json.Marshal(v)
	}
	return json.Marshal(v)
}
