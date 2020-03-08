package graphite

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/andreidm777/ago/cfg"
	"log"
)

var (
	graphite = cfg.String("collector.graphite_addr", "", "graphite addr (host:port) for sending stats")
	prefix   = cfg.String("collector.graphite_prefix", "notify_collector", "graphite prefix to be added before each point key")
)

type graphType struct {
	sync.Mutex
	addr *net.UDPAddr
}

var graph graphType

func (s *graphType) getGraphiteAddr() *net.UDPAddr {
	if s.addr != nil {
		return s.addr
	}

	s.Lock()
	defer s.Unlock()
	a, err := net.ResolveUDPAddr("udp", *graphite)
	if err != nil {
		log.Printf("[WARNING] Can't resolve graphite addr.")
		return nil
	}
	s.addr = a
	return s.addr
}

func SendStat(modPrefix string, stat string, count int) error {
	addr := graph.getGraphiteAddr()

	if addr == nil {
		return nil
	}

	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		log.Println("send statistics to graphite error: ", err)
		return err
	}
	defer conn.Close()

	_, err = fmt.Fprintf(conn, "%s.%s.%s %d %d", *prefix, modPrefix, stat, count, time.Now().Unix())
	if err != nil {
		log.Println("send statistics to graphite error: ", err)
		return err
	}
	return nil
}
