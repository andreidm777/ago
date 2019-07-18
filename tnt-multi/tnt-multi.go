package tntmulti

import (
	"errors"
	"github.com/tarantool/go-tarantool"
	"sync"
	"sync/atomic"
	"time"
)

const (
	connConnected  = iota
	connClosed
)

var (
	ErrEmptyAddrs = errors.New("addrs should not be empty")
	ErrWrongCheckTimeout = errors.New("wrong check timeout, must be greater than 0")
	ErrNoConnection = errors.New("no active connections")
)

type ConnectionMulti struct {
	addrs    []string
	connOpts tarantool.Opts
	opts     OptsMulti

	mutex    sync.RWMutex
	notify   chan tarantool.ConnEvent
	state    uint32
	control  chan struct{}
	pool     map[string]*tarantool.Connection
	fallback *tarantool.Connection
	current  int
}

var _ = tarantool.Connector(&ConnectionMulti{}) // check compatibility with connector interface

type OptsMulti struct {
	CheckTimeout time.Duration
}

func ConnectWithOpts(addrs []string, connOpts tarantool.Opts, opts OptsMulti) (connMulti *ConnectionMulti, err error) {
	if len(addrs) == 0 {
		return nil, ErrEmptyAddrs
	}
	if opts.CheckTimeout <= 0 {
		return nil, ErrWrongCheckTimeout
	}

	notify := make(chan tarantool.ConnEvent, 10 * len(addrs)) // x10 to accept disconnected and closed event (with a margin)
	connOpts.Notify = notify
	connMulti = &ConnectionMulti{
		addrs:    	addrs,
		connOpts: 	connOpts,
		opts:     	opts,
		notify:   	notify,
		control:  	make(chan struct{}),
		pool:		make(map[string]*tarantool.Connection),
	}
	somebodyAlive, _ := connMulti.warmUp()
	if !somebodyAlive {
		connMulti.Close()
		return nil, ErrNoConnection
	}
    go connMulti.checker()

	return connMulti, nil
}

func Connect(addrs []string, connOpts tarantool.Opts) (connMulti *ConnectionMulti, err error) {
	opts := OptsMulti{
		CheckTimeout: 1 * time.Second,
	}
	return ConnectWithOpts(addrs, connOpts, opts)
}

func (connMulti *ConnectionMulti) warmUp() (somebodyAlive bool, errs []error) {
	errs = make([]error, len(connMulti.addrs))

	for i, addr := range connMulti.addrs {
		conn, err := tarantool.Connect(addr, connMulti.connOpts)
		errs[i] = err
		if conn != nil && err == nil {
			if connMulti.fallback == nil {
				connMulti.fallback = conn
			}
			connMulti.pool[addr] = conn
			if conn.ConnectedNow() {
				somebodyAlive = true
			}
		}
	}
	return
}

func (connMulti *ConnectionMulti) getState() uint32 {
	return atomic.LoadUint32(&connMulti.state)
}

func (connMulti *ConnectionMulti) getConnectionFromPool(addr string) (*tarantool.Connection, bool) {
	connMulti.mutex.RLock()
	defer connMulti.mutex.RUnlock()
	conn, ok := connMulti.pool[addr]
	return conn, ok
}

func (connMulti *ConnectionMulti) setConnectionToPool(addr string, conn *tarantool.Connection) {
	connMulti.mutex.Lock()
	defer connMulti.mutex.Unlock()
	connMulti.pool[addr] = conn
}

func (connMulti *ConnectionMulti) deleteConnectionFromPool(addr string) {
	connMulti.mutex.Lock()
	defer connMulti.mutex.Unlock()
	delete(connMulti.pool, addr)
}

func (connMulti *ConnectionMulti) checker() {
	for connMulti.getState() != connClosed {
		timer := time.NewTimer(connMulti.opts.CheckTimeout)
		select {
		case <-connMulti.control:
			return
		case e := <-connMulti.notify:
			if connMulti.getState() == connClosed {
				return
			}
			if e.Conn.ClosedNow() {
				addr := e.Conn.Addr()
				if _, ok := connMulti.getConnectionFromPool(addr); !ok {
					continue
				}
				conn, _ := tarantool.Connect(addr, connMulti.connOpts)
				if conn != nil {
					connMulti.setConnectionToPool(addr, conn)
				} else {
					connMulti.deleteConnectionFromPool(addr)
				}
			}
		case <-timer.C:
			for _, addr := range connMulti.addrs {
				if connMulti.getState() == connClosed {
					return
				}
				if conn, ok := connMulti.getConnectionFromPool(addr); ok {
					if !conn.ClosedNow() {
						continue
					}
				}
				conn, _ := tarantool.Connect(addr, connMulti.connOpts)
				if conn != nil {
					connMulti.setConnectionToPool(addr, conn)
				}
			}
		}
	}
}

func (connMulti *ConnectionMulti) getCurrentConnection() *tarantool.Connection {
	connMulti.mutex.RLock()
	defer connMulti.mutex.RUnlock()
    
    if connMulti.fallback == nil {
        for i, addr := range connMulti.addrs {
            conn := connMulti.pool[addr]
            if conn != nil {
                if conn.ConnectedNow() {
                    return conn
                }
                connMulti.fallback = conn
                connMulti.current = i
            }
        }
    }
	return connMulti.fallback
}

func (connMulti *ConnectionMulti) Next() *ConnectionMulti {
	connMulti.mutex.RLock()
	defer connMulti.mutex.RUnlock()
    
    conn := connMulti.getConnectionByNum( connMulti.current + 1 )
    if conn == nil {
        connMulti.getConnectionByNum( 0 )
    }
        
    return connMulti
}

func (connMulti *ConnectionMulti) getConnectionByNum( num int ) *tarantool.Connection {
	connMulti.mutex.RLock()
	defer connMulti.mutex.RUnlock()

    if num >= len(connMulti.addrs) {
        return nil
    }

    conn := connMulti.pool[connMulti.addrs[num]]
	if conn != nil {
	    if conn.ConnectedNow() {
		    return conn
		}
		connMulti.fallback = conn
		connMulti.current  = num
	}
	return connMulti.fallback
}


func (connMulti *ConnectionMulti) ConnectedNow() bool {
	return connMulti.getState() == connConnected && connMulti.getCurrentConnection().ConnectedNow()
}

func (connMulti *ConnectionMulti) Close() (err error) {
	connMulti.mutex.Lock()
	defer connMulti.mutex.Unlock()

	close(connMulti.control)
	connMulti.state = connClosed

	for _, conn := range connMulti.pool {
		if err == nil {
			err = conn.Close()
		} else {
			conn.Close()
		}
	}
	if connMulti.fallback != nil {
		connMulti.fallback.Close()
	}

	return
}

func (connMulti *ConnectionMulti) Ping() (resp *tarantool.Response, err error) {
	return connMulti.getCurrentConnection().Ping()
}

func (connMulti *ConnectionMulti) ConfiguredTimeout() time.Duration {
	return connMulti.getCurrentConnection().ConfiguredTimeout()
}

func (connMulti *ConnectionMulti) Select(space, index interface{}, offset, limit, iterator uint32, key interface{}) (resp *tarantool.Response, err error) {
	return connMulti.getCurrentConnection().Select(space, index, offset, limit, iterator, key)
}

func (connMulti *ConnectionMulti) Insert(space interface{}, tuple interface{}) (resp *tarantool.Response, err error) {
	return connMulti.getCurrentConnection().Insert(space, tuple)
}

func (connMulti *ConnectionMulti) Replace(space interface{}, tuple interface{}) (resp *tarantool.Response, err error) {
	return connMulti.getCurrentConnection().Replace(space, tuple)
}

func (connMulti *ConnectionMulti) Delete(space, index interface{}, key interface{}) (resp *tarantool.Response, err error) {
	return connMulti.getCurrentConnection().Delete(space, index, key)
}

func (connMulti *ConnectionMulti) Update(space, index interface{}, key, ops interface{}) (resp *tarantool.Response, err error) {
	return connMulti.getCurrentConnection().Update(space, index, key, ops)
}

func (connMulti *ConnectionMulti) Upsert(space interface{}, tuple, ops interface{}) (resp *tarantool.Response, err error) {
	return connMulti.Upsert(space, tuple, ops)
}

func (connMulti *ConnectionMulti) Call(functionName string, args interface{}) (resp *tarantool.Response, err error) {
    resp, err = connMulti.getCurrentConnection().Call(functionName, args)
    if err != nil {
        for i := 0; i < len(connMulti.addrs); i++ {
            resp, err = connMulti.getConnectionByNum(i).Call(functionName, args)
            if err == nil {
                break
            }
        }
    }
	return resp, err
}

func (connMulti *ConnectionMulti) Call17(functionName string, args interface{}) (resp *tarantool.Response, err error) {
    resp, err = connMulti.getCurrentConnection().Call17(functionName, args)
    if err != nil {
        for i := 0; i < len(connMulti.addrs); i++ {
            resp, err = connMulti.getConnectionByNum(i).Call17(functionName, args)
            if err == nil {
                break
            }
        }
    }
	return resp, err
}

func (connMulti *ConnectionMulti) Eval(expr string, args interface{}) (resp *tarantool.Response, err error) {
	return connMulti.getCurrentConnection().Eval(expr, args)
}

func (connMulti *ConnectionMulti) GetTyped(space, index interface{}, key interface{}, result interface{}) (err error) {
	return connMulti.getCurrentConnection().GetTyped(space, index, key, result)
}

func (connMulti *ConnectionMulti) SelectTyped(space, index interface{}, offset, limit, iterator uint32, key interface{}, result interface{}) (err error) {
	return connMulti.getCurrentConnection().SelectTyped(space, index, offset, limit, iterator, key, result)
}

func (connMulti *ConnectionMulti) InsertTyped(space interface{}, tuple interface{}, result interface{}) (err error) {
	return connMulti.getCurrentConnection().InsertTyped(space, tuple, result)
}

func (connMulti *ConnectionMulti) ReplaceTyped(space interface{}, tuple interface{}, result interface{}) (err error) {
	return connMulti.getCurrentConnection().ReplaceTyped(space, tuple, result)
}

func (connMulti *ConnectionMulti) DeleteTyped(space, index interface{}, key interface{}, result interface{}) (err error) {
	return connMulti.getCurrentConnection().DeleteTyped(space, index, key, result)
}

func (connMulti *ConnectionMulti) UpdateTyped(space, index interface{}, key, ops interface{}, result interface{}) (err error) {
	return connMulti.getCurrentConnection().UpdateTyped(space, index, key, ops, result)
}

func (connMulti *ConnectionMulti) CallTyped(functionName string, args interface{}, result interface{}) (err error) {
	return connMulti.getCurrentConnection().CallTyped(functionName, args, result)
}

func (connMulti *ConnectionMulti) Call17Typed(functionName string, args interface{}, result interface{}) (err error) {
	return connMulti.getCurrentConnection().Call17Typed(functionName, args, result)
}

func (connMulti *ConnectionMulti) EvalTyped(expr string, args interface{}, result interface{}) (err error) {
	return connMulti.getCurrentConnection().EvalTyped(expr, args, result)
}

func (connMulti *ConnectionMulti) SelectAsync(space, index interface{}, offset, limit, iterator uint32, key interface{}) *tarantool.Future {
	return connMulti.getCurrentConnection().SelectAsync(space, index, offset, limit, iterator, key)
}

func (connMulti *ConnectionMulti) InsertAsync(space interface{}, tuple interface{}) *tarantool.Future {
	return connMulti.getCurrentConnection().InsertAsync(space, tuple)
}

func (connMulti *ConnectionMulti) ReplaceAsync(space interface{}, tuple interface{}) *tarantool.Future {
	return connMulti.getCurrentConnection().ReplaceAsync(space, tuple)
}

func (connMulti *ConnectionMulti) DeleteAsync(space, index interface{}, key interface{}) *tarantool.Future {
	return connMulti.getCurrentConnection().DeleteAsync(space, index, key)
}

func (connMulti *ConnectionMulti) UpdateAsync(space, index interface{}, key, ops interface{}) *tarantool.Future {
	return connMulti.getCurrentConnection().UpdateAsync(space, index, key, ops)
}

func (connMulti *ConnectionMulti) UpsertAsync(space interface{}, tuple interface{}, ops interface{}) *tarantool.Future {
	return connMulti.getCurrentConnection().UpsertAsync(space, tuple, ops)
}

func (connMulti *ConnectionMulti) CallAsync(functionName string, args interface{}) *tarantool.Future {
	return connMulti.getCurrentConnection().CallAsync(functionName, args)
}

func (connMulti *ConnectionMulti) Call17Async(functionName string, args interface{}) *tarantool.Future {
	return connMulti.getCurrentConnection().Call17Async(functionName, args)
}

func (connMulti *ConnectionMulti) EvalAsync(expr string, args interface{}) *tarantool.Future {
	return connMulti.getCurrentConnection().EvalAsync(expr, args)
}
