package cfg

import (
    "fmt"
    log "github.com/sirupsen/logrus"
    "io/ioutil"
	"os"
    "reflect"
	"gopkg.in/yaml.v3"
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

func typeBuild(v interface{}, val ParamValue) {
    switch val.t {
        case reflect.Int:
            i := val.v.(*int)
            *i = v.(int)
        case reflect.String:
            i := val.v.(*string)
            *i = v.(string)
        case reflect.Bool:
            i := val.v.(*bool)
            *i = v.(bool)
        case reflect.Slice:
            if i, ok := val.v.(*[]string) ; ok {
                if len(*i) > 0 {
                    *i = nil
                }
                for _, vi := range v.([]interface{}) {
                    *i = append(*i, vi.(string))
                }
            } else if i, ok := val.v.(*[]int) ; ok {
                if len(*i) > 0 {
                    *i = nil
                }
                for _, vi := range v.([]interface{}) {
                    *i = append(*i, vi.(int))
                }
            }
        default:
    }
}

func subParser(values *map[string]interface{}, prefix *string) {
    for k, v := range *values {
            if prefix != nil {
                k = fmt.Sprintf("%s.%s", *prefix, k)
            }
            if reflect.TypeOf(v).Kind() == reflect.Map {
                valuesIn, _ := v.(map[string]interface{})
                subParser(&valuesIn, &k)
            } else if val, ok := config.bindings[k]; ok {
                if reflect.TypeOf(v).Kind() == val.t {
                    typeBuild(v, val)
                }
            }
    }
}
        
func Parse(filename string) error {
    config.Fname = filename
	file, err   := os.Open(config.Fname)
	if err != nil {
        log.Warn(err)
		return err
	}
	
	defer file.Close()
	
	buffer, err := ioutil.ReadAll(file)
    if err != nil {
        log.Warn(err)
        return err
    }

    var values map[string]interface{}
    
    err = yaml.Unmarshal(buffer, &values)
    if err == nil {
        subParser(&values, nil)
    }
    log.Warn("parse success")
    return err
}

func Reload() {
    err := Parse(config.Fname)
    if err != nil {
        log.Warn(err)
    }
}

func AddBinding(name string, desc string, val interface{}) interface{} {
    if tmp, ok := config.bindings[name]; ok {
        if tmp.t == reflect.ValueOf(val).Elem().Kind() {
            return tmp.v
        } else {
            log.Fatalf("bad type %v %v", tmp.t, reflect.ValueOf(val).Elem().Kind())
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

func StringSlice(name string, defaultval []string, desc string) *[]string {
    v   := new([]string)
    ret := AddBinding(name, desc, v)
    v = ret.(*[]string)
    for _, val := range defaultval {
		*v = append(*v, val)
	}
    return v
}

func IntSlice(name string, defaultval []int, desc string) *[]int {
    v   := new([]int)
    ret := AddBinding(name, desc, v)
    v = ret.(*[]int)
    for _, val := range defaultval {
		*v = append(*v, val)
	}
    return v
}

func String(name string, defaultval string, desc string ) *string {
    v := new(string)
    ret := AddBinding(name, desc, v)
    
    v = ret.(*string)
    *v = defaultval
    return v
}

func Bool(name string, defaultval bool, desc string ) *bool {
    v := new(bool)
    ret := AddBinding(name, desc, v)
    
    v = ret.(*bool)
    *v = defaultval
    return v
}
