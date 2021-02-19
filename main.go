package main

import (
	"bufio"
	"flag"
	"fmt"
	"github.com/yl2chen/cidranger"
	"log"
	"net"
	"os"
	"sync"
	"time"
)

type rangeList struct {
	ranger cidranger.Ranger
	mutex  sync.RWMutex
}

func (r *rangeList) load(path string) bool {
	file, err := os.Open(path)

	if err != nil {
		log.Printf("Failed to open %s: %s", path, err)
		return false
	}

	defer file.Close()

	scanner := bufio.NewScanner(file)
	ranger := cidranger.NewPCTrieRanger()

	for scanner.Scan() {
		_, network, _ := net.ParseCIDR(scanner.Text())

		if network != nil {
			ranger.Insert(cidranger.NewBasicRangerEntry(*network))
		}
	}

	r.mutex.Lock()
	r.ranger = ranger
	r.mutex.Unlock()

	return true
}

func (r *rangeList) contains(ipstr string) bool {
	r.mutex.RLock()
	contains, _ := r.ranger.Contains(net.ParseIP(ipstr))
	r.mutex.RUnlock()

	return contains
}

func handleClient(client net.Conn, list *rangeList) {
	log.Printf("Client connected [%s]", client.RemoteAddr().Network())
	scanner := bufio.NewScanner(client)

	for scanner.Scan() {
		fmt.Println(scanner.Text())
		response := "0"

		if list.contains(scanner.Text()) {
			response = "1"
		}

		client.Write([]byte(response + "\n"))
	}

	client.Close()
	log.Println("Client disconnected")
}

func main() {
	socketPathPtr := flag.String("socket", "", "path to a unix socket")
	cidrsPathPtr := flag.String("cidrs", "", "path to cidrs file")
	refreshHoursPtr := flag.Int("refresh", 0, "refresh interval in hours")

	flag.Parse()

	if *socketPathPtr == "" || *cidrsPathPtr == "" {
		fmt.Println("Socket and CIDR paths required")
		flag.PrintDefaults()
		os.Exit(1)
	}

	var list rangeList

	if !list.load(*cidrsPathPtr) {
		os.Exit(1)
	}

	if *refreshHoursPtr > 0 {
		ticker := time.NewTicker(time.Duration(*refreshHoursPtr) * time.Hour)
		go func() {
			for {
				<-ticker.C

				if list.load(*cidrsPathPtr) {
					fmt.Println("Refreshed cidrs")
				}
			}
		}()
	}

	if err := os.RemoveAll(*socketPathPtr); err != nil {
		log.Fatal(err)
	}

	sock, err := net.Listen("unix", *socketPathPtr)

	if err != nil {
		log.Fatal("Listen failed: ", err)
	}

	defer sock.Close()

	for {
		client, err := sock.Accept()

		if err != nil {
			log.Fatal("Accept failed: ", err)
		}

		go handleClient(client, &list)
	}
}
