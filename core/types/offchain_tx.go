// Copyright 2021 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package types

import (
	"bytes"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
)

const OffchainTxType = 0x7D

type OffchainTx struct {
	// SourceHash uniquely identifies the source of the deposit
	SourceHash common.Hash
	// From is exposed through the types.Signer, not through TxData
	From common.Address
	// nil means contract creation
	To *common.Address `rlp:"nil"`
	// gas limit
	Gas uint64
	// Field indicating if this transaction is exempt from the L2 gas limit.
	IsSystemTransaction bool
	// Normal Tx data
	Data []byte
}

// copy creates a deep copy of the transaction data and initializes all fields.
func (tx *OffchainTx) copy() TxData {
	cpy := &OffchainTx{
		SourceHash:          tx.SourceHash,
		From:                tx.From,
		To:                  copyAddressPtr(tx.To),
		Gas:                 tx.Gas,
		IsSystemTransaction: tx.IsSystemTransaction,
		Data:                common.CopyBytes(tx.Data),
	}
	return cpy
}

// accessors for innerTx.
func (tx *OffchainTx) txType() byte           { return OffchainTxType }
func (tx *OffchainTx) chainID() *big.Int      { return common.Big0 }
func (tx *OffchainTx) accessList() AccessList { return nil }
func (tx *OffchainTx) data() []byte           { return tx.Data }
func (tx *OffchainTx) gas() uint64            { return tx.Gas }
func (tx *OffchainTx) gasFeeCap() *big.Int    { return new(big.Int) }
func (tx *OffchainTx) gasTipCap() *big.Int    { return new(big.Int) }
func (tx *OffchainTx) gasPrice() *big.Int     { return new(big.Int) }
func (tx *OffchainTx) value() *big.Int        { return new(big.Int) }
func (tx *OffchainTx) nonce() uint64          { return 0 }
func (tx *OffchainTx) to() *common.Address    { return tx.To }
func (tx *OffchainTx) blobGas() uint64           { return 0 }
func (tx *OffchainTx) blobGasFeeCap() *big.Int   { return nil }
func (tx *OffchainTx) blobHashes() []common.Hash { return nil }
func (tx *OffchainTx) isSystemTx() bool       { return tx.IsSystemTransaction }

func (tx *OffchainTx) effectiveGasPrice(dst *big.Int, baseFee *big.Int) *big.Int {
	return dst.Set(new(big.Int))
}

func (tx *OffchainTx) rawSignatureValues() (v, r, s *big.Int) {
	return common.Big0, common.Big0, common.Big0
}

func (tx *OffchainTx) setSignatureValues(chainID, v, r, s *big.Int) {
	// this is a noop for deposit transactions
}

func (tx *OffchainTx) encode(b *bytes.Buffer) error {
	return rlp.Encode(b, tx)
}

func (tx *OffchainTx) decode(input []byte) error {
	return rlp.DecodeBytes(input, tx)
}
