package tntmulti

import (
    "time"
    "log"
    "errors"
    "fmt"
    "github.com/andreidm777/ago/cfg"
    "github.com/tarantool/go-tarantool"
)

var Opts = tarantool.Opts{
	Timeout: 2000 * time.Millisecond,
}

var NumTry = cfg.Int("tarantool.try_connect", 3, "max queue for tarantool")

type MultiTnt struct {
    connsAddrs []string
    conns []*tarantool.Connection
    nowNum int
}

func New(addrs []string) *MultiTnt {
    num := len(addrs)
    ret := &MultiTnt{
        connsAddrs: addrs,
        conns: make([]*tarantool.Connection, num),
        nowNum: 0,
    }
    var err error
    for i := 0; i < num; i++ {
        ret.conns[i], err = tarantool.Connect(ret.connsAddrs[i], Opts)
        if err != nil {
			log.Printf("TERR[connect(%s)]: 1 (%v)", ret.connsAddrs[i], err)
		}
    }
    return ret
}

func (this *MultiTnt) getNextNode() {
    if this.conns[this.nowNum] != nil {
        this.conns[this.nowNum].Close()
    }
    this.conns[this.nowNum] = nil
    this.nowNum++
    if this.nowNum >= len(this.conns) {
        this.nowNum = 0
    }
}

func (this *MultiTnt) Call(function string, args []interface{})( *tarantool.Response, error ) {
    try := 0
    for try < *NumTry {
        try++
        var err_conn error
        if this.conns[this.nowNum] == nil {
            this.conns[this.nowNum], err_conn = tarantool.Connect(this.connsAddrs[this.nowNum], Opts)
            if err_conn != nil {
                log.Printf("TERR[connect(%s)]: 1 (%v)", this.connsAddrs[this.nowNum], err_conn)
                this.getNextNode()
                continue
            }
        }
       
        res, err := this.conns[this.nowNum].Call17(function, args)
        if err != nil || res.Code != tarantool.OkCode {
            log.Printf("TERR[call(%s)]: 1 %v", this.connsAddrs[this.nowNum], err)
            this.getNextNode()
            continue
		}
        
        if len(res.Data) > 1 {
            if reply, ok := res.Data[1].(string); ok {
                if reply == "MAINTENANCE" {
                    log.Printf("ret error %v", res)
                    this.getNextNode()
                    continue
                }
            }
        }
        return res, nil
	}
    return nil, errors.New(fmt.Sprintf("multi error for %s %v", function, args))
}
