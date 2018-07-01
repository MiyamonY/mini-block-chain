// Package main provides
//
// File:  main.go
// Author: ymiyamoto
//
// Created on Fri Jun 29 01:27:33 2018
//
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/MiyamonY/mini-block-chain/block"
	P2P "github.com/MiyamonY/mini-block-chain/p2p"

	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
)

const (
	HOST            = "127.0.0.1"
	API_PORT        = 3000
	P2P_PORT        = 4000
	INIT            = "/init/"
	BLOCKLIST       = "/blocks"
	BLOCK           = "/block"
	NODELIST        = "/nodes/"
	NODE            = "/node/"
	MALICIOUS_BLOCK = "/malicious_block"
	debugMode       = true
)

var (
	p2p *P2P.P2PNetwork
	bc  *block.BlockChain
)

func listBlocks(c echo.Context) error {
	log.Println("listBlocks:")
	blocks := bc.ListBlock()
	return c.JSON(http.StatusOK, blocks)
}

func getBlock(c echo.Context) error {
	id := c.Param("id")
	log.Println("getBlock:", id)

	block := bc.GetBlockByData([]byte(id))

	if block != nil {
		return c.JSON(http.StatusOK, block)
	}
	index, err := strconv.Atoi(id)
	if err == nil {
		block := bc.GetBlockByIndex(index)
		if block != nil {
			return c.JSON(http.StatusOK, block)
		}
	}
	block = bc.GetBlock(id)
	if block != nil {
		return c.JSON(http.StatusOK, block)
	}
	return echo.NewHTTPError(http.StatusNotFound, "Block is not found.id ="+id)
}

func listNodes(c echo.Context) error {
	log.Println("listNodes:")
	node := p2p.List()
	return c.JSON(http.StatusOK, node)
}

type Data struct {
	Data string `json:"data"`
}

func createBlock(c echo.Context) error {
	fmt.Println("createBlock:")
	if bc.IsMining() {
		return echo.NewHTTPError(http.StatusConflict, "Already Mining")
	}

	data := &Data{}
	if err := c.Bind(data); err != nil {
		fmt.Println(err)
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid data info.")
	}
	bc.SaveData([]byte(data.Data))
	return c.NoContent(http.StatusOK)
}

func addNode(c echo.Context) error {
	fmt.Println("addNode:")
	node := &P2P.Node{}

	if err := c.Bind(node); err != nil {
		fmt.Println(err)
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid server info.")
	}

	id, err := p2p.Add(node)
	if debugMode {
		log.Println(id)
		log.Println(err)
	}
	return c.NoContent(http.StatusOK)
}

func initBlockChain(c echo.Context) error {
	id := c.Param("id")
	fmt.Println("initBlockChain: ", id)
	index, err := strconv.Atoi(id)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid Hight.")
	}
	bc.SyncBlockChain(index)
	return c.NoContent(http.StatusOK)
}

type BlockModify struct {
	Hight int    `json:"hight"`
	Data  string `json:"data"`
}

func maliciousBlock(c echo.Context) error {
	log.Print("maliciousBlock:")
	data := &BlockModify{}

	if err := c.Bind(data); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid data.")
	}
	bc.Modify(data.Hight, data.Data)
	return c.NoContent(http.StatusOK)
}

func requestHandler(c echo.Context) error {
	return c.String(http.StatusOK, "Mini Block Cahin ver 0.1")
}

func main() {
	apiport := flag.Int("apiport", API_PORT, "API port number")
	p2pport := flag.Int("p2pport", P2P_PORT, "P2P port number")
	host := flag.String("host", HOST, "host name")
	first := flag.Bool("first", false, "first server")
	flag.Parse()

	apiPort := uint16(*apiport)
	p2pPort := uint16(*p2pport)
	myHost := *host
	log.Println("HOST:", myHost)
	log.Println("API PORT:", apiPort)
	log.Println("P2P PORT:", p2pPort)

	p2p, err := P2P.New(myHost, apiPort, p2pPort)
	if err != nil {
		log.Fatalf("%+v", err)
	}

	bc, err := block.New(p2p, true)
	if err != nil {
		log.Fatalf("%+v", err)
	}

	if *first {
		bc.Initialized()
	}
	if debugMode {
		log.Printf("%+v", p2p)
		log.Printf("%+v", bc)
	}

	p2p.SetAction(P2P.CMD_NEWBLOCK, bc.NewBlock)
	p2p.SetAction(P2P.CMD_ADDSRV, p2p.AddServer)
	p2p.SetAction(P2P.CMD_SENDBLOCK, bc.SendBlock)
	p2p.SetAction(P2P.CMD_MININGBOCK, bc.MiningBlock)
	p2p.SetAction(P2P.CMD_MODIFYDATA, bc.ModifyData)

	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{echo.GET, echo.PUT, echo.POST, echo.DELETE, echo.HEAD},
	}))

	e.GET("/", requestHandler)
	e.GET(BLOCKLIST, listBlocks)
	e.GET(BLOCK+":id", getBlock)
	e.POST(BLOCK, createBlock)
	e.GET(NODELIST, listNodes)
	e.POST(NODE, addNode)
	e.PUT(NODE, addNode)
	e.POST(MALICIOUS_BLOCK, maliciousBlock)
	e.POST(INIT+":id", initBlockChain)

	e.Logger.Fatal(e.Start(myHost + ":" + strconv.FormatInt(int64(apiPort), 10)))
}
