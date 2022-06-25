package main

import (
	"log"
)

func init() {
	log.SetPrefix("Blockchain: ")
}

func main() {

	bc := newBlockchain()

	bc.AddTransaction("A", "B", 20)
	bc.CreateBlock(0, bc.LastBlock().Hash())

	bc.AddTransaction("A", "B", 10)
	bc.AddTransaction("C", "B", 12)
	bc.CreateBlock(4, bc.LastBlock().Hash())

	bc.Print()
}
