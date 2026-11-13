// altunwan-probe: timestamped TCP probe between containers.
//
//	altunwan-probe serve <port>            TCP echo server (one line in, same line out)
//	altunwan-probe rtt <host:port> [n]     n round trips: TCP connect time and 64-byte echo
//	                                       RTT; prints min / median / max in ms.
package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"sort"
	"strconv"
	"time"
)

func serve(port string) {
	l, err := net.Listen("tcp", ":"+port)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for {
		c, err := l.Accept()
		if err != nil {
			continue
		}
		go func(c net.Conn) {
			defer c.Close()
			r := bufio.NewReader(c)
			for {
				line, err := r.ReadBytes('\n')
				if err != nil {
					return
				}
				c.Write(line)
			}
		}(c)
	}
}

func stats(v []float64) string {
	sort.Float64s(v)
	return fmt.Sprintf("min %.2f med %.2f max %.2f", v[0], v[len(v)/2], v[len(v)-1])
}

func rtt(addr string, n int) {
	var conn, echo []float64
	msg := []byte(fmt.Sprintf("%063d\n", 0))
	buf := make([]byte, len(msg))
	for i := 0; i < n; i++ {
		t0 := time.Now()
		c, err := net.DialTimeout("tcp", addr, 5*time.Second)
		if err != nil {
			fmt.Println("error", err)
			os.Exit(1)
		}
		conn = append(conn, float64(time.Since(t0).Microseconds())/1000)
		c.(*net.TCPConn).SetNoDelay(true)
		t1 := time.Now()
		c.Write(msg)
		if _, err := bufioReadFull(c, buf); err != nil {
			fmt.Println("error", err)
			os.Exit(1)
		}
		echo = append(echo, float64(time.Since(t1).Microseconds())/1000)
		c.Close()
		time.Sleep(20 * time.Millisecond)
	}
	fmt.Printf("%s connect_ms %s | echo_ms %s\n", addr, stats(conn), stats(echo))
}

func bufioReadFull(c net.Conn, b []byte) (int, error) {
	got := 0
	for got < len(b) {
		k, err := c.Read(b[got:])
		got += k
		if err != nil {
			return got, err
		}
	}
	return got, nil
}

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: altunwan-probe serve <port> | rtt <host:port> [n]")
		os.Exit(2)
	}
	switch os.Args[1] {
	case "serve":
		serve(os.Args[2])
	case "rtt":
		n := 10
		if len(os.Args) > 3 {
			n, _ = strconv.Atoi(os.Args[3])
		}
		rtt(os.Args[2], n)
	}
}
