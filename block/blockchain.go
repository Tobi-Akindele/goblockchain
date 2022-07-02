package block

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"goblockchain/utils"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	MINING_DIFFICULTY = 3
	MINING_SENDER     = "THE BLOCKCHAIN"
	MINING_REWARD     = 1.0
	MINING_TIMER_SEC  = 20

	BLOCKCHAIN_PORT_RANGE_START        = 5001
	BLOCKCHAIN_PORT_RANGE_END          = 5003
	NEIGHBOUR_IP_RANGE_START           = 0
	NEIGHBOUR_IP_RANGE_END             = 1
	BLOCKCHAIN_NEIGHBOUR_SYNC_TIME_SEC = 20
)

type Block struct {
	Nonce        int            `json:"nonce"`
	PreviousHash [32]byte       `json:"previousHash"`
	Timestamp    int64          `json:"timestamp"`
	Transactions []*Transaction `json:"transactions"`
}

func (b *Block) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Nonce        int            `json:"nonce"`
		PreviousHash string         `json:"previousHash"`
		Timestamp    int64          `json:"timestamp"`
		Transactions []*Transaction `json:"transactions"`
	}{
		Nonce:        b.Nonce,
		PreviousHash: fmt.Sprintf("%x", b.PreviousHash),
		Timestamp:    b.Timestamp,
		Transactions: b.Transactions,
	})
}

func newBlock(nonce int, previousHash [32]byte, transactions []*Transaction) *Block {
	b := new(Block)
	b.Timestamp = time.Now().UnixNano()
	b.Nonce = nonce
	b.PreviousHash = previousHash
	b.Transactions = transactions
	return b
}

func (b *Block) GetPreviousHash() [32]byte {
	return b.PreviousHash
}

func (b *Block) GetNonce() int {
	return b.Nonce
}

func (b *Block) GetTransactions() []*Transaction {
	return b.Transactions
}

func (b *Block) Hash() [32]byte {
	m, _ := json.Marshal(b)
	return sha256.Sum256(m)
}

func (b *Block) Print() {
	fmt.Printf("PreviousHash      %x\n", b.PreviousHash)
	fmt.Printf("Nonce             %d \n", b.Nonce)
	fmt.Printf("Timestamp         %d \n", b.Timestamp)
	fmt.Println("Transactions: ")
	for _, t := range b.Transactions {
		t.Print()
	}
}

func (b *Block) UnmarshalJSON(data []byte) error {
	var previousHash string
	v := &struct {
		Timestamp    *int64          `json:"timestamp"`
		Nonce        *int            `json:"nonce"`
		PreviousHash *string         `json:"previousHash"`
		Transactions *[]*Transaction `json:"transactions"`
	}{
		Timestamp:    &b.Timestamp,
		Nonce:        &b.Nonce,
		PreviousHash: &previousHash,
		Transactions: &b.Transactions,
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	ph, _ := hex.DecodeString(*v.PreviousHash)
	copy(b.PreviousHash[:], ph[:32])
	return nil
}

type Blockchain struct {
	TransactionPool   []*Transaction `json:"transactionPool"`
	Chain             []*Block       `json:"chain"`
	BlockChainAddress string         `json:"blockChainAddress"`
	Port              uint16         `json:"port"`
	mux               sync.Mutex

	neighbours    []string
	muxNeighbours sync.Mutex
}

func NewBlockchain(blockChainAddress string, port uint16) *Blockchain {
	b := &Block{}
	bc := new(Blockchain)
	bc.BlockChainAddress = blockChainAddress
	bc.Port = port
	bc.CreateBlock(0, b.Hash())
	return bc
}

func (bc *Blockchain) Run() {
	bc.StartSyncNeighbours()
	bc.ResolveConflicts()
	bc.StartMining()
}

func (bc *Blockchain) SetNeighbours() {
	bc.neighbours = utils.FindNeighbours(
		utils.GetHost(), bc.Port, NEIGHBOUR_IP_RANGE_START, NEIGHBOUR_IP_RANGE_END,
		BLOCKCHAIN_PORT_RANGE_START, BLOCKCHAIN_PORT_RANGE_END)
	log.Printf("%v", bc.neighbours)
}

func (bc *Blockchain) SyncNeighbours() {
	bc.muxNeighbours.Lock()
	defer bc.muxNeighbours.Unlock()
	bc.SetNeighbours()
}

func (bc *Blockchain) StartSyncNeighbours() {
	bc.SyncNeighbours()
	_ = time.AfterFunc(time.Second*BLOCKCHAIN_NEIGHBOUR_SYNC_TIME_SEC, bc.StartSyncNeighbours)
}

func (bc *Blockchain) GetTransactionPool() []*Transaction {
	return bc.TransactionPool
}

func (bc *Blockchain) ClearTransactionPool() {
	bc.TransactionPool = bc.TransactionPool[:0]
}

func (bc *Blockchain) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Blocks []*Block `json:"chain"`
	}{
		Blocks: bc.Chain,
	})
}

func (bc *Blockchain) UnmarshalJSON(data []byte) error {
	v := &struct {
		Blocks *[]*Block `json:"chain"`
	}{
		Blocks: &bc.Chain,
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	return nil
}

func (bc *Blockchain) CreateBlock(nonce int, previousHash [32]byte) *Block {
	block := newBlock(nonce, previousHash, bc.TransactionPool)
	bc.Chain = append(bc.Chain, block)
	bc.TransactionPool = []*Transaction{}

	for _, n := range bc.neighbours {
		endpoint := fmt.Sprintf("http://%s/transactions", n)
		client := &http.Client{}
		req, _ := http.NewRequest("DELETE", endpoint, nil)
		resp, _ := client.Do(req)
		log.Printf("%v", resp)
	}

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

func (t *Transaction) UnmarshalJSON(data []byte) error {
	v := &struct {
		Sender    *string  `json:"senderBlockchainAddress"`
		Recipient *string  `json:"recipientBlockchainAddress"`
		Value     *float32 `json:"value"`
	}{
		Sender:    &t.SenderBlockchainAddress,
		Recipient: &t.RecipientBlockchainAddress,
		Value:     &t.Value,
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	return nil
}

func (bc *Blockchain) CreateTransaction(sender string, recipient string, value float32, senderPublicKey *ecdsa.PublicKey, s *utils.Signature) bool {
	isTransacted := bc.AddTransaction(sender, recipient, value, senderPublicKey, s)

	if isTransacted {
		for _, n := range bc.neighbours {
			publicKeyStr := fmt.Sprintf("%064x%064x", senderPublicKey.X.Bytes(), senderPublicKey.Y.Bytes())
			signatureStr := s.String()
			bt := &TransactionRequest{
				SenderBlockchainAddress:    &sender,
				RecipientBlockchainAddress: &recipient,
				SenderPublicKey:            &publicKeyStr,
				Value:                      &value,
				Signature:                  &signatureStr,
			}
			m, _ := json.Marshal(bt)
			buf := bytes.NewBuffer(m)
			endpoint := fmt.Sprintf("http://%s/transactions", n)
			client := &http.Client{}
			req, _ := http.NewRequest("PUT", endpoint, buf)
			resp, _ := client.Do(req)
			log.Printf("%v", resp)
		}
	}

	return isTransacted
}

func (bc *Blockchain) AddTransaction(sender string, recipient string, value float32, senderPublicKey *ecdsa.PublicKey, s *utils.Signature) bool {
	t := NewTransaction(sender, recipient, value)

	if sender == MINING_SENDER {
		bc.TransactionPool = append(bc.TransactionPool, t)
		return true
	}

	if bc.VerifyTransactionSignature(senderPublicKey, s, t) {
		if bc.CalculateTotalAmount(sender) < value {
			log.Println("ERROR: Insufficient balance")
			return false
		}
		bc.TransactionPool = append(bc.TransactionPool, t)
		return true
	}
	log.Println("ERROR: Verify Transaction")
	return false
}

func (bc *Blockchain) VerifyTransactionSignature(senderPublicKey *ecdsa.PublicKey, s *utils.Signature, t *Transaction) bool {
	m, _ := json.Marshal(t)
	h := sha256.Sum256(m)
	return ecdsa.Verify(senderPublicKey, h[:], s.R, s.S)
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

func (bc *Blockchain) ProofOfWork() int {
	transactions := bc.CopyTransactionPool()
	previousHash := bc.LastBlock().Hash()
	nonce := 0
	for !bc.ValidProof(nonce, previousHash, transactions, MINING_DIFFICULTY) {
		nonce += 1
	}
	return nonce
}

func (bc *Blockchain) Mining() bool {
	bc.mux.Lock()
	defer bc.mux.Unlock()

	//if len(bc.TransactionPool) == 0 {
	//	return false
	//}

	bc.AddTransaction(MINING_SENDER, bc.BlockChainAddress, MINING_REWARD, nil, nil)
	nonce := bc.ProofOfWork()
	previousHash := bc.LastBlock().Hash()
	bc.CreateBlock(nonce, previousHash)
	log.Println("action=mining, status=success")

	for _, n := range bc.neighbours {
		endpoint := fmt.Sprintf("http://%s/consensus", n)
		client := &http.Client{}
		req, _ := http.NewRequest("PUT", endpoint, nil)
		resp, _ := client.Do(req)
		log.Printf("%v", resp)
	}

	return true
}

func (bc *Blockchain) StartMining() {
	bc.Mining()
	_ = time.AfterFunc(time.Second*MINING_TIMER_SEC, bc.StartMining)
}

func (bc *Blockchain) CalculateTotalAmount(blockchainAddress string) float32 {
	var totalAmount float32 = 0.0000
	for _, b := range bc.Chain {
		for _, t := range b.Transactions {
			value := t.Value
			if blockchainAddress == t.RecipientBlockchainAddress {
				totalAmount += value
			}
			if blockchainAddress == t.SenderBlockchainAddress {
				totalAmount -= value
			}
		}
	}
	return totalAmount
}

func (bc *Blockchain) ValidChain(chain []*Block) bool {
	preBlock := chain[0]
	currentIndex := 1
	for currentIndex < len(chain) {
		b := chain[currentIndex]
		if b.PreviousHash != preBlock.Hash() {
			return false
		}

		if !bc.ValidProof(b.GetNonce(), b.GetPreviousHash(), b.GetTransactions(), MINING_DIFFICULTY) {
			return false
		}

		preBlock = b
		currentIndex += 1
	}
	return true
}

func (bc *Blockchain) ResolveConflicts() bool {
	var longestChain []*Block = nil
	maxLength := len(bc.Chain)

	for _, n := range bc.neighbours {
		endpoint := fmt.Sprintf("http://%s/chain", n)
		resp, _ := http.Get(endpoint)
		if resp.StatusCode == 200 {
			var bcResp Blockchain
			decoder := json.NewDecoder(resp.Body)
			_ = decoder.Decode(&bcResp)

			chain := bcResp.Chain

			if len(chain) > maxLength && bc.ValidChain(chain) {
				maxLength = len(chain)
				longestChain = chain
			}
		}
	}

	if longestChain != nil {
		bc.Chain = longestChain
		log.Println("Resolve conflicts replaced")
		return true
	}
	log.Println("Resolve conflicts not replaced")
	return false
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

type TransactionRequest struct {
	SenderBlockchainAddress    *string  `json:"sender_blockchain_address"`
	RecipientBlockchainAddress *string  `json:"recipient_blockchain_address"`
	SenderPublicKey            *string  `json:"sender_public_key"`
	Value                      *float32 `json:"value"`
	Signature                  *string  `json:"signature"`
}

func (tr *TransactionRequest) ValidateTransactionRequest() bool {
	if tr.Signature == nil || tr.SenderBlockchainAddress == nil ||
		tr.RecipientBlockchainAddress == nil || tr.SenderPublicKey == nil ||
		tr.Value == nil {
		return false
	}
	return true
}

type AmountResponse struct {
	Amount float32 `json:"amount"`
}
