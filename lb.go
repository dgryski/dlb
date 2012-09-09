package main

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"flag"
	msgpack "github.com/ugorji/go-msgpack"
	"hash/crc32"
	"log"
	"math/rand"
	"net"
	"net/rpc"
	"os"
	"strconv"
	"sync"
)

type Machine struct {
	Addr       string
	Weight     int
	packedAddr [6]byte
}

type Pool struct {
	Service     string
	serviceCrc  uint32
	Machines    []*Machine
	PackPort    int
	totalWeight int
	mu          sync.RWMutex
}

type Cfg struct {
	Port  int
	Pools []*Pool
}

func packAddr(dst []byte, addr string) {
	host, port, _ := net.SplitHostPort(addr)
	ip := net.ParseIP(host)
	portnum, _ := strconv.Atoi(port)

	copy(dst[:], ip.To4())
	binary.BigEndian.PutUint16(dst[4:], uint16(portnum))
}

// straight-forward implementation
// speedups: http://en.wikipedia.org/wiki/Alias_method
func (p *Pool) selectMachine() *Machine {

	p.mu.RLock()

	r := rand.Int31n(int32(p.totalWeight))

	for _, m := range p.Machines {
		r -= int32(m.Weight)
		if r < 0 {
			p.mu.RUnlock()
			return m
		}
	}
	panic("broken rand")
}

type LBNS struct {
	pools map[string]*Pool
}

var errUnknownService = errors.New("unknown service")

func (l *LBNS) Req(service string, addr *string) error {

	p := l.pools[service]

	if p == nil {
		return errUnknownService
	}

	m := p.selectMachine()
	*addr = m.Addr

	return nil
}

type SetParam struct {
	Service string
	Addr    string
	Weight  uint
}

func (l *LBNS) Set(param SetParam, result *string) error {

	p := l.pools[param.Service]

	if p == nil {
		return errUnknownService
	}

	p.mu.Lock()

	// see if the machine we're adding exists
	var m *Machine
	for _, machine := range p.Machines {
		if param.Addr == machine.Addr {
			m = machine
			break
		}
	}

	if m == nil {
		m = new(Machine)
		m.Addr = param.Addr
		packAddr(m.packedAddr[:], m.Addr)
		p.Machines = append(p.Machines, m)
	}

	p.totalWeight += (int(param.Weight) - m.Weight)
	m.Weight = int(param.Weight)

	p.mu.Unlock()

	*result = "ok"

	return nil
}

func (l *LBNS) List(service string, result *string) error {

	p := l.pools[service]

	if p == nil {
		return errUnknownService
	}

	var err error

	p.mu.RLock()
	buf, err := json.Marshal(p)
	p.mu.RUnlock()

	*result = string(buf)

	return err
}

func main() {

	var optCfgFile = flag.String("cfg", "", "config file")

	flag.Parse()

	if *optCfgFile == "" {
		log.Fatal("no config file found")
	}

	cfgFile, err := os.Open(*optCfgFile)
	if err != nil {
		log.Fatal(err)
	}

	dec := json.NewDecoder(cfgFile)

	var cfg Cfg

	if err := dec.Decode(&cfg); err != nil {
		log.Fatal(err)
	}

	pools := make(map[string]*Pool)

	for _, pool := range cfg.Pools {
		for _, m := range pool.Machines {
			pool.totalWeight += m.Weight
			packAddr(m.packedAddr[:], m.Addr)
		}

		log.Println("service=", []byte(pool.Service))

		pool.serviceCrc = crc32.ChecksumIEEE([]byte(pool.Service))

		pools[pool.Service] = pool

		log.Printf("pool: %s (%x) loaded\n", pool.Service, pool.serviceCrc)
	}

        /*
        // tcp support
	for _, pool := range pools {

		if pool.PackPort != 0 {
			go func(pool *Pool) {
				ln, e := net.Listen("tcp", ":"+strconv.Itoa(pool.PackPort))
				if e != nil {
					log.Fatal("listen error:", e)
				}

				log.Println("tcp server starting")

				for {
					conn, err := ln.Accept()
					if err != nil {
						log.Println(err)
						continue
					}
					go func(c net.Conn) {
						m := cfg.Pools[0].selectMachine()
						c.Write([]byte(m.packedAddr[:]))
						c.Close()
					}(conn)
				}
			}(pool)
		}
	}
        */

	go func(port int, pools map[string]*Pool) {

		pconn, e := net.ListenPacket("udp", ":"+strconv.Itoa(port))
		if e != nil {
			log.Fatal("udp listen error:", e)
		}

		log.Println("udp server starting on port", port)

		for {
			var b [255]byte
			n, addr, err := pconn.ReadFrom(b[:])
			if err != nil {
				log.Println(err)
				continue
			}

			log.Println("packet from: ", addr)

			if n != int(b[0])+1 {
				log.Println("bad packet: n=", n, "b[0]=", b[0])
			}

			s := string(b[1:n])

			log.Println("lookup request for service=", s)

			p := pools[s]

			go func(pconn net.PacketConn, addr net.Addr, service string, p *Pool) {

				var b [10]byte

				reqCrc := crc32.ChecksumIEEE([]byte(service))
				binary.LittleEndian.PutUint32(b[:], reqCrc)

				if p != nil {
					m := p.selectMachine()
					copy(b[4:], m.packedAddr[:])
				} else {
					log.Println("service lookup failed -- returning error")
				}

				log.Println("returning bytes: ", b)

				n, err := pconn.WriteTo(b[:], addr)

				if n != 10 || err != nil {
					log.Println("error sending packet: ", n, "/10 bytes written, err=", err)
				}
			}(pconn, addr, s, p)
		}
	}(cfg.Port, pools)

	// msgpack rpc server
	lbns := &LBNS{pools}
	rpc.Register(lbns)

	ln, e := net.Listen("tcp", ":50000")
	if e != nil {
		log.Fatal("listen error:", e)
	}

	log.Println("rpc server starting")

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Println(err)
			continue
		}
		c := msgpack.NewCustomRPCServerCodec(conn, nil)

		go rpc.ServeCodec(c)
	}
}
