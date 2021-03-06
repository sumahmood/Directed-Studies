package main

import (
	"fmt"
	"bytes"
	"github.com/btcsuite/btcd/database"
	_ "github.com/btcsuite/btcd/database/ffldb"
	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/mining/cpuminer"
	"github.com/btcsuite/btcd/mining"
	"github.com/btcsuite/btcd/mempool"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcutil"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"os"
	"path/filepath"
	"time"
)


func main() {
	dbPath := filepath.Join(os.TempDir(), "exampleprocessblock")
	_ = os.RemoveAll(dbPath)
	db, err := database.Create("ffldb", dbPath, chaincfg.SimNetParams.Net)
	if err != nil {
		fmt.Printf("Failed to create database: %v\n", err)
		return
	}
	defer os.RemoveAll(dbPath)
	defer db.Close()

	// Create a new BlockChain instance for a simulation test bitcoin network
	chain, err := blockchain.New(&blockchain.Config{
		DB:          db,
		ChainParams: &chaincfg.SimNetParams,
		TimeSource:  blockchain.NewMedianTime(),
	})
	if err != nil {
		fmt.Printf("Failed to create chain instance: %v\n", err)
		return
	}

	// Generate address and pay to it
	addrHash := [20]byte{0x01}

	addr, err := btcutil.NewAddressPubKeyHash(addrHash[:],
		&chaincfg.SimNetParams)
	if err != nil {
		fmt.Printf("NewAddressPubKeyHash: unexpected error: %v\n", err)
	}
	dummyPkScript, err := txscript.PayToAddrScript(addr)
	if err != nil {
		fmt.Printf("PayToAddrScript: unexpected error: %v\n", err)
	}
	dummyTxOut := wire.TxOut{
		Value:    100,
		PkScript: dummyPkScript,
	}

	// Generating block template
	amt, err := btcutil.NewAmount(1000.0)
	if err != nil {
		fmt.Printf("New amount: Error generating amount: %v\n", err)
	}

	// Creating a new transaction.
    str := wire.NewMsgTx(1) // wire/msgtx.go

    // Received from btcd/mempool/policy_test.go
    prevOutHash, err := chainhash.NewHashFromStr("01abcd134")
	if err != nil {
		fmt.Printf("NewShaHashFromStr: unexpected error: %v\n", err)
	}	
    dummyPrevOut := wire.OutPoint{Hash: *prevOutHash, Index: 1}
	dummySigScript := bytes.Repeat([]byte{0x00}, 65)
	dummyTxIn := wire.TxIn{
		PreviousOutPoint: dummyPrevOut,
		SignatureScript:  dummySigScript,
		Sequence:         1,
	}

	// Add TxIn and TxOut values to generate a new transaction
	str.AddTxIn(&dummyTxIn)
	str.AddTxOut(&dummyTxOut)

	trans2 := btcutil.NewTx(str)
	
	miningPolicy := mining.Policy{BlockMinWeight: 1, BlockMaxWeight: 1000, BlockMinSize: 1, BlockMaxSize: 2048, BlockPrioritySize: 100, TxMinFreeFee: amt}
	// from btcd/mempool/mempool_test.go. Sets transaction pool policy.
	txPoolPolicy := mempool.Policy{
				DisableRelayPriority: true,
				FreeTxRelayLimit:     15.0,
				MaxOrphanTxs:         5,
				MaxOrphanTxSize:      1000,
				MaxSigOpCostPerTx:    blockchain.MaxBlockSigOpsCost / 4,
				MinRelayTxFee:        1000, // 1 Satoshi per byte
				MaxTxVersion:         1,
			}
	configPoolPolicy := &mempool.Config{
			Policy: txPoolPolicy,
			ChainParams:      &chaincfg.SimNetParams,
			FetchUtxoView:    chain.FetchUtxoView,
			BestHeight:       func() int32 {return chain.BestSnapshot().Height},
			MedianTimePast:   func() time.Time {return chain.BestSnapshot().MedianTime},
			CalcSequenceLock: func(tx *btcutil.Tx, utxoView *blockchain.UtxoViewpoint) (*blockchain.SequenceLock, error) {return chain.CalcSequenceLock(tx, utxoView, true)},
			SigCache:         nil,
			AddrIndex:        nil,
		}

	// Creates a new transaction pool which, along with the mining policy is use to generate a block templates
	txPool := mempool.New(configPoolPolicy)
	txPool.MaybeAcceptTransaction(trans2, true, false) // returns chainHash, and txdescription (add those values later)
	medianTime := blockchain.NewMedianTime()
	sigCache := txscript.NewSigCache(100)
	hashCache := txscript.NewHashCache(100)
	templateGen := mining.NewBlkTmplGenerator(&miningPolicy, &chaincfg.SimNetParams, txPool, chain, medianTime, sigCache, hashCache)

	valNew := [1]btcutil.Address{addr}

	// This creates a CPU miner that mines blocks based on the block template and the transaction pool policy
	cpuMiner := cpuminer.New(&cpuminer.Config{
		ChainParams:            &chaincfg.SimNetParams,
		BlockTemplateGenerator: templateGen,
		MiningAddrs:            valNew[:],
		ProcessBlock:          	func(block *btcutil.Block, bflags blockchain.BehaviorFlags) (bool, error) {
			_, a, b := chain.ProcessBlock(block, bflags) 
			return a, b},
		ConnectedCount:         func() int32 {return 1},
		IsCurrent:              func() bool {return true},
	})

	// Uses the CPU miner to generate 2 blocks
	cpuMiner.GenerateNBlocks(2)
}