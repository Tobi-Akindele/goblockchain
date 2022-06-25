package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const MINING_DIFFICULTY = 3

type Block struct {
	Nonce        int            `json:"nonce"`
	PreviousHash [32]byte       `json:"previousHash"`
	Timestamp    int64          `json:"timestamp"`
	Transactions []*Transaction `json:"transactions"`
}

func newBlock(nonce int, previousHash [32]byte, transactions []*Transaction) *Block {
	b := new(Block)
	b.Timestamp = time.Now().UnixNano()
	b.Nonce = nonce
	b.PreviousHash = previousHash
	b.Transactions = transactions
	return b
}

func (b *Block) Hash() [32]byte {
	m, _ := json.Marshal(b)
	return sha256.Sum256(m)
}

func (b *Block) Print() {
	fmt.Printf("PreviousHash      %x\n", b.PreviousHash)
	fmt.Printf("Nonce             %d \n", b.Nonce)
	fmt.Printf("Timestamp         %d \n", b.Timestamp)
	for _, t := range b.Transactions {
		t.Print()
	}
}

type Blockchain struct {
	TransactionPool []*Transaction `json:"transactionPool"`
	Chain           []*Block       `json:"chain"`
}

func newBlockchain() *Blockchain {
	b := &Block{}
	bc := new(Blockchain)
	bc.CreateBlock(0, b.Hash())
	return bc
}

func (bc *Blockchain) CreateBlock(nonce int, previousHash [32]byte) *Block {
	block := newBlock(nonce, previousHash, bc.TransactionPool)
	bc.Chain = append(bc.Chain, block)
	bc.TransactionPool = []*Transaction{}
	return block
}

func (bc *Blockchain) LastBlock() *Block {
	return bc.Chain[len(bc.Chain)-1]
}

type Transaction struct {
	SenderBlockchainAddress    string  `json:"senderBlockchainAddress"`
	RecipientBlockchainAddress string  `json:"recipientBlockchainAddress"`
	Value                      float32 `json:"value"`
}

func (bc *Blockchain) AddTransaction(sender string, recipient string, value float32) {
	t := NewTransaction(sender, recipient, value)
	bc.TransactionPool = append(bc.TransactionPool, t)
}

func (bc *Blockchain) CopyTransactionPool() []*Transaction {
	transactions := make([]*Transaction, 0)
	for _, t := range bc.TransactionPool {
		transactions = append(transactions, NewTransaction(t.SenderBlockchainAddress, t.RecipientBlockchainAddress, t.Value))
	}
	return transactions
}

func (bc *Blockchain) ValidProof(nonce int, previousHash [32]byte, transactions []*Transaction, difficulty int) bool {
	zeros := strings.Repeat("0", difficulty)
	guessBlock := Block{
		Nonce:        nonce,
		PreviousHash: previousHash,
		Timestamp:    0,
		Transactions: transactions,
	}
	guessHashStr := fmt.Sprintf("%x", guessBlock.Hash())
	return guessHashStr[:3] == zeros
}

func NewTransaction(sender string, recipient string, value float32) *Transaction {
	return &Transaction{
		SenderBlockchainAddress:    sender,
		RecipientBlockchainAddress: recipient,
		Value:                      value,
	}
}

func (t *Transaction) Print() {
	fmt.Printf("%s\n", strings.Repeat("-", 40))
	fmt.Printf(" senderBlockchainAddress       %s\n", t.SenderBlockchainAddress)
	fmt.Printf(" recipientBlockchainAddress    %s\n", t.RecipientBlockchainAddress)
	fmt.Printf(" value                         %.4f\n", t.Value)
}

func (bc *Blockchain) Print() {
	fmt.Printf("%s \n", strings.Repeat("*", 25))
	for i, block := range bc.Chain {
		fmt.Printf("%s Chain %d %s \n", strings.Repeat("=", 25), i, strings.Repeat("=", 25))
		block.Print()
	}
	fmt.Printf("%s \n", strings.Repeat("*", 25))
}
