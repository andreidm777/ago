package cfg

import (
    "fmt"
    //"reflect"
    "testing"
    log "github.com/sirupsen/logrus"
    "log/syslog"
)

var i = Int("test", 1, "test")

var a = IntSlice("test_1", []int{1,2}, "test 1")
var c = Bool("test_2", false, "test 1")
var d = String("test_3", "test", "test 1")

func TestConf(t *testing.T) {
    logwriter, e := syslog.New(syslog.LOG_NOTICE, "myprog")
    if e == nil {
        log.SetOutput(logwriter)
    }
    Parse("test.yaml")
    fmt.Printf("%d\n", *i)
    fmt.Println(*a)
    fmt.Println(*c)
    fmt.Println(*d)
}
