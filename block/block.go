// Package block provides
//
// File:  block.go
// Author: ymiyamoto
//
// Created on Sun Jul  1 04:56:14 2018
//
package block

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/MiyamonY/mini-block-chain/p2p"
)

const (
	MaxPowCount = 60
	Difficulty  = "0"
)

var (
	debugMode = true
)

type Block struct {
	Hight     int    `json:"hight"`
	Prev      string `json:"prev"`
	Hash      string `json:"hash"`
	Nonce     string `json:"nonce"`
	PowCount  int    `json:"powcount"`
	Data      string `json:"data"`
	TimeStamp int64  `json:"timestamp"`
	Child     []*Block
	Sibling   []*Block
}

func (b *Block) calcHash() string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("%d%s%s%d%s%d", b.Hight, b.Prev, b.Nonce, b.PowCount, b.Data, b.TimeStamp))))
}

func (b *Block) hash() string {
	b.Hash = b.calcHash()
	return b.Hash
}

func (b *Block) isValid() bool {
	if debugMode {
		log.Printf("Hash = %s", b.Hash)
		log.Printf("calc Hash = %s", b.calcHash())
	}
	if b.Hash != b.calcHash() {
		return false
	}
	return true
}

type BlockChain struct {
	Info          string
	p2p           *p2p.P2PNetwork
	mining        bool
	initialized   bool
	info          string
	blocks        []*Block
	invalidBlocks []*Block
	orphanBlocks  []*Block
	mu            sync.Mutex
}

func New(p2p *p2p.P2PNetwork, first bool) (*BlockChain, error) {
	log.Printf("Block New")
	bc := &BlockChain{p2p: p2p, initialized: false, mining: false, info: "mini block chain v0.1"}
	bc.blocks = make([]*Block, 0)
	bc.invalidBlocks = make([]*Block, 0)
	bc.orphanBlocks = make([]*Block, 0)

	if first {
		block := &Block{Hight: 0, TimeStamp: 0, Data: "Genesis Block"}
		block.Child = make([]*Block, 0)
		block.Sibling = make([]*Block, 0)
		block.hash()
		bc.blocks = append(bc.blocks, block)
	}

	return bc, nil
}

func (bc *BlockChain) IsMining() bool {
	return bc.mining
}

func (bc *BlockChain) SyncBlockChain(hight int) error {
	log.Printf("sync block chain")
	if err := bc.RequestBlock(hight); err != nil {
		log.Printf("RequestBlock() error. %s", err.Error())
		return err
	}
	time.Sleep(1 * time.Second)
	bc.initialized = true
	return nil
}

func (bc *BlockChain) Initialized() error {
	bc.initialized = true
	return nil
}

func (bc *BlockChain) IsInitialized() bool {
	return bc.initialized
}

func (bc *BlockChain) Create(data string, pow bool, primary bool) (*Block, error) {
	log.Printf("Create: %s", data)

	lastBlock := bc.getPrevBlock()
	block := &Block{Prev: lastBlock.Hash, TimeStamp: time.Now().UnixNano(), Data: data, Hight: lastBlock.Hight + 1,
		Child: make([]*Block, 0), Sibling: make([]*Block, 0)}
	if pow {
		for i := 0; i < MaxPowCount; i++ {
			block.Nonce = fmt.Sprintf("%x", rand.New(rand.NewSource(block.TimeStamp/int64(i+1))).Int63())
			block.PowCount = i
			block.hash()
			if debugMode {
				log.Printf("Try %d times %+v", i, block)
			} else {
				log.Printf("Try %d times %s", i, block.Hash)
			}

			if strings.HasPrefix(block.Hash, Difficulty) {
				log.Printf("found!")
				break
			}
			time.Sleep(1 * time.Second)
		}

		if primary == false && !strings.HasPrefix(block.Hash, Difficulty) {
			return nil, errors.New("failed to mine")
		}
	} else {
		block.hash()
	}

	return block, nil
}

func (bc *BlockChain) Check(data []byte) error {
	log.Printf("Checking my Block Chian ...")
	bc.mu.Lock()
	defer bc.mu.Unlock()

	prev := bc.blocks[0]
	for i := 1; i < len(bc.blocks); i++ {
		log.Printf("check %d", i)
		if prev.calcHash() != bc.blocks[i].Prev {
			log.Printf("Invalid block found. %+v", prev)
			bc.invalidBlocks = append(bc.invalidBlocks, prev)
		}
		prev = bc.blocks[i]
	}

	log.Printf("Check Done")
	return nil
}

func (bc *BlockChain) AddBlock(block *Block) error {
	if debugMode {
		log.Printf("AddBlock: %+v", block)
	}

	bc.mu.Lock()
	defer bc.mu.Unlock()

	if err := bc.blockAppendSimple(block); err != nil {
		log.Printf("blockAppendSimple() error. %s", err.Error())
		return err
	}

	lastBlock := bc.blocks[len(bc.blocks)-1]
	for i, b := range bc.orphanBlocks {
		if b.Prev == lastBlock.Prev {
			if debugMode {
				log.Printf("reappend orphan Block.")
				log.Printf("list block. before")
				bc.DumpChain()
			}

			bc.orphanBlocks = append(bc.orphanBlocks[:i], bc.orphanBlocks[i+1:]...)

			if err := bc.blockAppendSimple(b); err != nil {
				log.Printf("blockAppendSimple() error. %s", err.Error())
				return err
			}

			if debugMode {
				log.Printf("list blcok after")
				bc.DumpChain()
			}
		}
	}

	return nil
}

func (bc *BlockChain) RequestBlock(id int) error {
	log.Printf("RequestBlock : %d", id)

	bid := make([]byte, 4)
	binary.LittleEndian.PutUint32(bid, uint32(id))
	node := []byte(bc.p2p.Self())
	sMsg := append(bid, node...)
	log.Printf("Request Block: %s", string(sMsg))
	bc.p2p.SendOne(p2p.CMD_SENDBLOCK, sMsg)
	return nil
}

func (bc *BlockChain) GetBlock(hash string) *Block {
	log.Printf("GetBlock: %s", hash)
	bc.mu.Lock()
	defer bc.mu.Unlock()

	for _, b := range bc.blocks {
		log.Printf("%+v", b)
		if b.Hash == hash {
			log.Printf("GetBlock() found: %+v", b)
			return b
		}
	}
	return nil
}

func (bc *BlockChain) GetBlockByIndex(index int) *Block {
	log.Printf("GetBlockByIndex : %d", index)

	bc.mu.Lock()
	defer bc.mu.Unlock()

	if index < len(bc.blocks) {
		b := bc.blocks[index]
		log.Printf("GetBlockByIndex found: %+v", b)
		return b
	}

	return nil
}

func (bc *BlockChain) GetBlockByData(data []byte) *Block {
	log.Printf("GetBlockByData : %s", string(data))

	bc.mu.Lock()
	defer bc.mu.Unlock()
	for _, b := range bc.blocks {
		if b.Data == string(data) {
			log.Printf("GetBlockByData found: %+v", b)
			return b
		}
	}

	return nil
}

func (bc *BlockChain) ListBlock() []*Block {
	log.Printf("ListBlock()\n Block list")
	bc.mu.Lock()
	defer bc.mu.Unlock()

	for i, b := range bc.blocks {
		log.Printf("%d: %+v", i, b)
	}
	log.Printf("Orphan blocks")
	for i, b := range bc.orphanBlocks {
		log.Printf("%d: %+v", i, b)
	}

	return bc.blocks
}

func (bc *BlockChain) DumpChain() {
	log.Printf("%+v", bc)
}

func (bc *BlockChain) NewBlock(msg []byte) error {
	log.Printf("new block action")

	block := &Block{}
	if err := json.Unmarshal(msg, block); err != nil {
		log.Printf("unmarshal error. %s", err.Error())
		return err
	}

	if debugMode {
		log.Printf("new block %+v", block)
	}

	if !block.isValid() {
		return fmt.Errorf("invalid block id:%d", block.Hight)
	}

	return bc.AddBlock(block)
}

func (bc *BlockChain) SendBlock(msg []byte) error {
	log.Printf("send block action")
	if debugMode {
		log.Printf("msg : %s", string(msg))
	}

	var blockID uint32
	buf := bytes.NewReader(msg)
	if err := binary.Read(buf, binary.LittleEndian, &blockID); err != nil {
		log.Printf("invalid block id: %s", err.Error())
		return err
	}

	target := string(msg[4:])
	log.Printf("id %d, target: %s", blockID, target)

	block := bc.GetBlockByIndex(int(blockID))
	if block == nil {
		log.Printf("invalid block id: %d", blockID)
		return fmt.Errorf("invalid block ID: %d", blockID)
	}

	server := strings.Split(target, ":")
	port, err := strconv.Atoi(server[1])
	if err != nil {
		log.Printf("invalid port num. %s", err.Error())
		return err
	}

	node := bc.p2p.Search(server[0], uint16(port))
	if node == nil {
		log.Printf("node not found at %s. %s", target, err.Error())
		return err
	}

	b, err := json.Marshal(node)
	if err != nil {
		log.Printf("json marshal error. %s", err.Error())
		return err
	}
	sMsg := append([]byte{byte(p2p.CMD_NEWBLOCK)}, b...)

	return node.Send(sMsg)
}

func (bc *BlockChain) MiningBlock(data []byte) error {
	log.Printf("MiningBlock()")
	return bc.miningBlock(data, false)
}

func (bc *BlockChain) miningBlock(data []byte, primary bool) error {
	log.Printf("miningBlock()")

	if err := bc.startMining(); err != nil {
		log.Printf("%s", err.Error())
		return err
	}

	log.Printf("start mining")

	d := data
	if block, err := bc.Create(string(d), true, primary); err == nil {
		b, err := json.Marshal(block)
		if err != nil {
			log.Printf("json marshal error")
			return err
		}
		bc.p2p.Broadcast(p2p.CMD_NEWBLOCK, b, true)
	}

	bc.stopMining()
	log.Printf("mining done")

	return nil
}

func (bc *BlockChain) SaveData(data []byte) error {
	log.Printf("SaveData() save data %s", string(data))

	bc.p2p.Broadcast(p2p.CMD_MININGBLOCK, data, false)

	go bc.miningBlock(data, true)

	return nil
}

func (bc *BlockChain) ModifyData(msg []byte) error {
	log.Printf("modifiy data %s", string(msg))

	var hight uint32
	buf := bytes.NewReader(msg)
	if err := binary.Read(buf, binary.LittleEndian, &hight); err != nil {
		return errors.New("invalid request")
	}

	data := string(msg[4:])
	if debugMode {
		log.Printf("data: %s", string(data))
	}

	block := &Block{}
	target := bc.GetBlockByIndex(int(hight))
	if target == nil {
		return fmt.Errorf("no target block : id = %d", hight)
	}

	*block = *target
	block.Data = data
	if !block.isValid() {
		log.Printf("invalid block: %+v", block)
		return fmt.Errorf("invalid block : id= %d", hight)
	}

	*target = *block
	d := make([]byte, 4)

	return bc.Check(d)
}

func (bc *BlockChain) Modify(hight int, data string) error {
	log.Printf("modify : %d, %s", hight, data)
	index := make([]byte, 4)
	binary.LittleEndian.PutUint32(index, uint32(hight))

	sMsg := append(index, data...)
	log.Printf("sMsg = %s", sMsg)
	bc.p2p.Broadcast(p2p.CMD_MODIFYDATA, sMsg, true)
	return nil
}

func (bc *BlockChain) getPrevBlock() *Block {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	block := bc.blocks[len(bc.blocks)-1]
	longest := block
	if len(block.Sibling) > 0 {
		for _, b := range block.Sibling {
			if len(block.Child) < len(b.Child) {
				longest = b
			}
		}
	}

	return longest
}

func (bc *BlockChain) startMining() error {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	if !bc.initialized {
		log.Printf("block is uninitialized. can't start mining")
		return errors.New("can't start mining")
	}

	if bc.mining {
		log.Printf("already mining")
		return errors.New("already mining")
	}

	bc.mining = true
	return nil
}

func (bc *BlockChain) stopMining() {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	bc.mining = false
}

func (bc *BlockChain) blockAppendSimple(block *Block) (err error) {
	if debugMode {
		log.Printf("blockAppendSimple:%+v", block)
	}

	lastBlock := bc.blocks[len(bc.blocks)-1]
	if block.Prev == lastBlock.Hash {
		bc.blocks = append(bc.blocks, block)
		return nil
	}

	if lastBlock.Prev == block.Prev {
		if lastBlock.TimeStamp > block.TimeStamp {
			bc.blocks[len(bc.blocks)-1] = block
			log.Printf("Replace and purge block: %+v", lastBlock)
		}
		return
	}

	if block.Hight > lastBlock.Hight {
		bc.orphanBlocks = append(bc.orphanBlocks, block)
		for i := lastBlock.Hight + 1; i < block.Hight; i++ {
			if err = bc.RequestBlock(i); err != nil {
				log.Printf("request block error %s", err.Error())
				return
			}
			time.Sleep(1 * time.Second / 2)
		}
		return
	}

	log.Printf("Purge Block: %+v", block)
	return
}
