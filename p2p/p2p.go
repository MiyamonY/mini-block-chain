// Package p2p provides
//
// File:  p2p.go
// Author: ymiyamoto
//
// Created on Sat Jun 30 19:40:01 2018
//
package p2p

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"time"
)

const (
	CMD_NEWBLOCK = iota
	CMD_ADDSRV
	CMD_DELSRV
	CMD_SENDBLOCK
	CMD_MININGBOCK
	CMD_MODIFYDATA
)

var (
	debugMode = true
)

type Node struct {
	Host    string   `json:"host" query:"host"`
	APIPort uint16   `json:"api_port" query:"api_port" `
	P2PPort uint16   `json:"p2p_port" query:"p2p_port" `
	Self    bool     `json:"-"`
	Conn    net.Conn `json:"-"`
}

func (node *Node) connect() {
	target := fmt.Sprintf("%s:%d", node.Host, node.P2PPort)
	if debugMode {
		log.Printf("target = %s\n", target)
	}

	conn, err := net.Dial("udp", target)
	if err != nil {
		log.Printf("faild to connect %s. %s\n", target, err.Error())
		node.Conn = nil
	} else {
		log.Printf("%s connected.\n", target)
		node.Conn = conn
	}
}

func (node *Node) disconnect() {
	if node.Conn != nil {
		node.Conn.Close()
	}
}

func (node *Node) Send(msg []byte) (err error) {
	log.Printf("Send to %s : %s, %d", node.me(), string(msg), len(msg))

	if node.Conn != nil {
		n, err := node.Conn.Write(msg)
		if err != nil {
			log.Printf("Write err: %+v %s", n, err.Error())
		}
	} else {
		err = fmt.Errorf("Not connectd : %s", node.me())
		log.Printf("Not connectd %s", node.me())
	}

	return err
}

func (node *Node) me() string {
	return fmt.Sprintf("%s:%d", node.Host, node.P2PPort)
}

type ActFn func([]byte) error

type P2PNetwork struct {
	nodes   []*Node
	actions []ActFn
}

func (p2p *P2PNetwork) Self() string {
	for _, n := range p2p.nodes {
		if n.Self {
			return n.me()
		}
	}
	return ""
}

func (p2p *P2PNetwork) P2PServ(host string, port uint16) {
	log.Printf("Start P2P server %s:%d", host, port)

	udpAddr := &net.UDPAddr{
		IP:   net.ParseIP(host),
		Port: int(port),
	}

	udpLn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		log.Printf("listen err %s", err.Error())
		return
	}

	for {
		buf := make([]byte, 1024)
		n, addr, err := udpLn.ReadFromUDP(buf)
		if err != nil {
			log.Printf("udp error. %s", err.Error())
			return
		}

		if debugMode {
			log.Printf("call udpLn.ReadFromUDP")
			log.Printf("read %d.", n)
		}

		go func() {
			cmd := int(buf[0])
			msg := buf[1:n]

			if debugMode {
				log.Printf("Receive action request. %s from %+v ", string(buf), addr)
				log.Printf("Cmd %d", cmd)
			}

			if cmd < len(p2p.actions) {
				log.Print("Do Actions")
				f := p2p.actions[cmd]
				if f != nil {
					err := f(msg)
					if err != nil {
						log.Printf("Action error %s", err.Error())
					}
				} else {
					log.Printf("No actions")
				}
			} else {
				log.Printf("Invalid action")
			}
		}()
	}
}

func (p2p *P2PNetwork) Add(node *Node) (int, error) {
	log.Print("P2P Network Add.")
	log.Printf("add node : %+v", node)

	bytes, err := json.Marshal(node)
	if err != nil {
		log.Printf("json Marshal error. %s", err.Error())
		return 0, err
	}

	p2p.Broadcast(CMD_ADDSRV, bytes, false)
	node.connect()

	for _, n := range p2p.nodes {
		b, _ := json.Marshal(n)
		sMsg := append([]byte{byte(CMD_ADDSRV)}, b...)
		if err := node.Send(sMsg); err != nil {
			log.Printf("node.Send() error. %s", err.Error())
		}
		time.Sleep(1 * time.Second / 2)
	}
	p2p.nodes = append(p2p.nodes, node)
	return 0, nil
}

func (p2p *P2PNetwork) Search(host string, p2pPort uint16) *Node {
	log.Printf("Search: %s:%d", host, p2pPort)

	for _, node := range p2p.nodes {
		if node.Host == host && node.P2PPort == p2pPort {
			return node
		}
	}
	return nil
}

func (p2p *P2PNetwork) List() []*Node {
	for _, node := range p2p.nodes {
		log.Printf("%+v", node)
	}
	return p2p.nodes
}

func (p2p *P2PNetwork) Broadcast(cmd int, msg []byte, self bool) {
	log.Printf("Broadcast : cmd %d, msg %s", cmd, string(msg))
	sMsg := append([]byte{byte(cmd)}, msg...)
	if debugMode {
		log.Print(sMsg)
	}

	for _, node := range p2p.nodes {
		if debugMode {
			log.Print(node)
		}

		if self == false && node.Self {
			log.Printf("not send")
			continue
		}

		if err := node.Send(sMsg); err != nil {
			log.Printf("send error. %s", err.Error())
		}
		time.Sleep(1 * time.Second / 2)
	}
}

func (p2p *P2PNetwork) SendOne(cmd int, msg []byte) {
	log.Printf("SendOne: cmd %d, msg %s", cmd, string(msg))
	sMsg := append([]byte{byte(cmd)}, msg...)
	if debugMode {
		log.Print(sMsg)
	}

	for _, node := range p2p.nodes {
		if node.Self {
			log.Printf("not send")
			continue
		}

		if err := node.Send(sMsg); err != nil {
			break
		}
		time.Sleep(1 * time.Second / 2)
	}
}

func (p2p *P2PNetwork) SetAction(cmd int, handler ActFn) *ActFn {
	fn := p2p.actions[cmd]
	p2p.actions[cmd] = handler
	return &fn
}

func (p2p *P2PNetwork) AddServer(msg []byte) error {
	log.Printf("server add action")
	node := &Node{}

	if err := json.Unmarshal(msg, node); err != nil {
		log.Printf("jon.Unmarshal failed")
		return err
	}

	log.Printf("node: %+v", node)
	node.Self = false
	node.connect()
	p2p.nodes = append(p2p.nodes, node)
	return nil
}

func New(host string, apiPort, p2pPort uint16) (*P2PNetwork, error) {
	log.Printf("initalize P2PNetwork")

	node := &Node{Host: host, APIPort: apiPort, P2PPort: p2pPort, Self: true}
	node.connect()

	p2p := &P2PNetwork{}
	p2p.nodes = make([]*Node, 0)
	p2p.actions = make([]ActFn, 20)
	p2p.nodes = append(p2p.nodes, node)
	go p2p.P2PServ(host, p2pPort)

	if debugMode {
		log.Printf("%+v", p2p)
	}

	return p2p, nil
}
