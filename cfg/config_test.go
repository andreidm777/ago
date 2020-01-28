package cfg

import (
    "fmt"
    "testing"
)

var i = Int("test", 1, "test")

func TestConf(t *testing.T) {
    Parse("test.yaml")
    fmt.Printf("%d\n", *i)
}
