package cfg

import (
    "io/ioutil"
	"os"
    "reflect"
	"gopkg.in/yaml.v2"
)

type ParamValue struct {
    v interface{}
    t reflect.Kind
}

type Config struct {
	Fname string
	bindings map[string]ParamValue
	descriptions map[string]string
}

func create() *Config {
	return &Config{
        bindings:    make(map[string]ParamValue),
        descriptions: make(map[string]string),
	}
}

var config = create()

func Parse(filename string) error {
    config.Fname = filename
	file, err   := os.Open(config.Fname)
	if err != nil {
		return err
	}
	
	defer file.Close()
	
	buffer, err := ioutil.ReadAll(file)
    if err != nil {
        return err
    }

    var values map[string]interface{}
    
    err = yaml.Unmarshal(buffer, &values)
    if err == nil {
        for k, v := range values {
            if val, ok := config.bindings[k]; ok {
                if reflect.TypeOf(v).Kind() == val.t {
                    switch val.t {
                        case reflect.Int:
                            i := val.v.(*int)
                            *i = v.(int)                
                        default:
                    }
                }
            }
        }
    }
    return err
}

func AddBinding(name string, desc string, val interface{}) interface{} {
    if tmp, ok := config.bindings[name]; ok {
        if tmp.t == reflect.ValueOf(val).Elem().Kind() {
            return tmp.v
        } else {
            panic("bad type")
        }
    } else {
        config.bindings[name]     = ParamValue{ v:val, t: reflect.ValueOf(val).Elem().Kind() }
        config.descriptions[name] = desc
    }
    return val
}

func Int(name string, defaultval int, desc string ) *int {
    v := new(int)
    ret := AddBinding(name, desc, v)
    
    v = ret.(*int)
    *v = defaultval
    return v
}
