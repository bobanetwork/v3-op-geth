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
	"encoding/json"
	"errors"
	"io"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/holiman/uint256"
)

// txJSON is the JSON representation of transactions.
type txJSON struct {
	Type hexutil.Uint64 `json:"type"`

	ChainID              *hexutil.Big    `json:"chainId,omitempty"`
	Nonce                *hexutil.Uint64 `json:"nonce"`
	To                   *common.Address `json:"to"`
	Gas                  *hexutil.Uint64 `json:"gas"`
	GasPrice             *hexutil.Big    `json:"gasPrice"`
	MaxPriorityFeePerGas *hexutil.Big    `json:"maxPriorityFeePerGas"`
	MaxFeePerGas         *hexutil.Big    `json:"maxFeePerGas"`
	MaxFeePerDataGas     *hexutil.Big    `json:"maxFeePerDataGas,omitempty"`
	Value                *hexutil.Big    `json:"value"`
	Input                *hexutil.Bytes  `json:"input"`
	AccessList           *AccessList     `json:"accessList,omitempty"`
	BlobVersionedHashes  []common.Hash   `json:"blobVersionedHashes,omitempty"`
	V                    *hexutil.Big    `json:"v"`
	R                    *hexutil.Big    `json:"r"`
	S                    *hexutil.Big    `json:"s"`

	// Deposit transaction fields
	SourceHash *common.Hash    `json:"sourceHash,omitempty"`
	From       *common.Address `json:"from,omitempty"`
	Mint       *hexutil.Big    `json:"mint,omitempty"`
	IsSystemTx *bool           `json:"isSystemTx,omitempty"`

	// Only used for encoding:
	Hash common.Hash `json:"hash"`
}

// MarshalJSON marshals as JSON with a hash.
func (tx *Transaction) MarshalJSON() ([]byte, error) {
	var enc txJSON
	// These are set for all tx types.
	enc.Hash = tx.Hash()
	enc.Type = hexutil.Uint64(tx.Type())

	// Other fields are set conditionally depending on tx type.
	switch itx := tx.inner.(type) {
	case *LegacyTx:
		enc.Nonce = (*hexutil.Uint64)(&itx.Nonce)
		enc.To = tx.To()
		enc.Gas = (*hexutil.Uint64)(&itx.Gas)
		enc.GasPrice = (*hexutil.Big)(itx.GasPrice)
		enc.Value = (*hexutil.Big)(itx.Value)
		enc.Input = (*hexutil.Bytes)(&itx.Data)
		enc.V = (*hexutil.Big)(itx.V)
		enc.R = (*hexutil.Big)(itx.R)
		enc.S = (*hexutil.Big)(itx.S)

	case *AccessListTx:
		enc.ChainID = (*hexutil.Big)(itx.ChainID)
		enc.Nonce = (*hexutil.Uint64)(&itx.Nonce)
		enc.To = tx.To()
		enc.Gas = (*hexutil.Uint64)(&itx.Gas)
		enc.GasPrice = (*hexutil.Big)(itx.GasPrice)
		enc.Value = (*hexutil.Big)(itx.Value)
		enc.Input = (*hexutil.Bytes)(&itx.Data)
		enc.AccessList = &itx.AccessList
		enc.V = (*hexutil.Big)(itx.V)
		enc.R = (*hexutil.Big)(itx.R)
		enc.S = (*hexutil.Big)(itx.S)

	case *DynamicFeeTx:
		enc.ChainID = (*hexutil.Big)(itx.ChainID)
		enc.Nonce = (*hexutil.Uint64)(&itx.Nonce)
		enc.To = tx.To()
		enc.Gas = (*hexutil.Uint64)(&itx.Gas)
		enc.MaxFeePerGas = (*hexutil.Big)(itx.GasFeeCap)
		enc.MaxPriorityFeePerGas = (*hexutil.Big)(itx.GasTipCap)
		enc.Value = (*hexutil.Big)(itx.Value)
		enc.Input = (*hexutil.Bytes)(&itx.Data)
		enc.AccessList = &itx.AccessList
		enc.V = (*hexutil.Big)(itx.V)
		enc.R = (*hexutil.Big)(itx.R)
		enc.S = (*hexutil.Big)(itx.S)

	case *BlobTx:
		enc.ChainID = (*hexutil.Big)(itx.ChainID.ToBig())
		enc.Nonce = (*hexutil.Uint64)(&itx.Nonce)
		enc.Gas = (*hexutil.Uint64)(&itx.Gas)
		enc.MaxFeePerGas = (*hexutil.Big)(itx.GasFeeCap.ToBig())
		enc.MaxPriorityFeePerGas = (*hexutil.Big)(itx.GasTipCap.ToBig())
		enc.MaxFeePerDataGas = (*hexutil.Big)(itx.BlobFeeCap.ToBig())
		enc.Value = (*hexutil.Big)(itx.Value.ToBig())
		enc.Input = (*hexutil.Bytes)(&itx.Data)
		enc.AccessList = &itx.AccessList
		enc.BlobVersionedHashes = itx.BlobHashes
		enc.To = tx.To()
		enc.V = (*hexutil.Big)(itx.V.ToBig())
		enc.R = (*hexutil.Big)(itx.R.ToBig())
		enc.S = (*hexutil.Big)(itx.S.ToBig())

	case *DepositTx:
		enc.Gas = (*hexutil.Uint64)(&itx.Gas)
		enc.Value = (*hexutil.Big)(itx.Value)
		enc.Input = (*hexutil.Bytes)(&itx.Data)
		enc.To = tx.To()
		enc.SourceHash = &itx.SourceHash
		enc.From = &itx.From
		if itx.Mint != nil {
			enc.Mint = (*hexutil.Big)(itx.Mint)
		}
		enc.IsSystemTx = &itx.IsSystemTransaction
		// other fields will show up as null.
	}
	return json.Marshal(&enc)
}

// UnmarshalJSON unmarshals from JSON.
func (tx *Transaction) UnmarshalJSON(input []byte) error {
	var dec txJSON
	if err := json.Unmarshal(input, &dec); err != nil {
		return err
	}

	// Decode / verify fields according to transaction type.
	var inner TxData
	switch dec.Type {
	case LegacyTxType:
		var itx LegacyTx
		inner = &itx
		if dec.Nonce == nil {
			return errors.New("missing required field 'nonce' in transaction")
		}
		itx.Nonce = uint64(*dec.Nonce)
		if dec.To != nil {
			itx.To = dec.To
		}
		if dec.Gas == nil {
			return errors.New("missing required field 'gas' in transaction")
		}
		itx.Gas = uint64(*dec.Gas)
		if dec.GasPrice == nil {
			return errors.New("missing required field 'gasPrice' in transaction")
		}
		itx.GasPrice = (*big.Int)(dec.GasPrice)
		if dec.Value == nil {
			return errors.New("missing required field 'value' in transaction")
		}
		itx.Value = (*big.Int)(dec.Value)
		if dec.Input == nil {
			return errors.New("missing required field 'input' in transaction")
		}
		itx.Data = *dec.Input
		if dec.V == nil {
			return errors.New("missing required field 'v' in transaction")
		}
		itx.V = (*big.Int)(dec.V)
		if dec.R == nil {
			return errors.New("missing required field 'r' in transaction")
		}
		itx.R = (*big.Int)(dec.R)
		if dec.S == nil {
			return errors.New("missing required field 's' in transaction")
		}
		itx.S = (*big.Int)(dec.S)
		withSignature := itx.V.Sign() != 0 || itx.R.Sign() != 0 || itx.S.Sign() != 0
		if withSignature {
			if err := sanityCheckSignature(itx.V, itx.R, itx.S, true); err != nil {
				return err
			}
		}

	case AccessListTxType:
		var itx AccessListTx
		inner = &itx
		if dec.ChainID == nil {
			return errors.New("missing required field 'chainId' in transaction")
		}
		itx.ChainID = (*big.Int)(dec.ChainID)
		if dec.Nonce == nil {
			return errors.New("missing required field 'nonce' in transaction")
		}
		itx.Nonce = uint64(*dec.Nonce)
		if dec.To != nil {
			itx.To = dec.To
		}
		if dec.Gas == nil {
			return errors.New("missing required field 'gas' in transaction")
		}
		itx.Gas = uint64(*dec.Gas)
		if dec.GasPrice == nil {
			return errors.New("missing required field 'gasPrice' in transaction")
		}
		itx.GasPrice = (*big.Int)(dec.GasPrice)
		if dec.Value == nil {
			return errors.New("missing required field 'value' in transaction")
		}
		itx.Value = (*big.Int)(dec.Value)
		if dec.Input == nil {
			return errors.New("missing required field 'input' in transaction")
		}
		itx.Data = *dec.Input
		if dec.V == nil {
			return errors.New("missing required field 'v' in transaction")
		}
		if dec.AccessList != nil {
			itx.AccessList = *dec.AccessList
		}
		itx.V = (*big.Int)(dec.V)
		if dec.R == nil {
			return errors.New("missing required field 'r' in transaction")
		}
		itx.R = (*big.Int)(dec.R)
		if dec.S == nil {
			return errors.New("missing required field 's' in transaction")
		}
		itx.S = (*big.Int)(dec.S)
		withSignature := itx.V.Sign() != 0 || itx.R.Sign() != 0 || itx.S.Sign() != 0
		if withSignature {
			if err := sanityCheckSignature(itx.V, itx.R, itx.S, false); err != nil {
				return err
			}
		}

	case DynamicFeeTxType:
		var itx DynamicFeeTx
		inner = &itx
		if dec.ChainID == nil {
			return errors.New("missing required field 'chainId' in transaction")
		}
		itx.ChainID = (*big.Int)(dec.ChainID)
		if dec.Nonce == nil {
			return errors.New("missing required field 'nonce' in transaction")
		}
		itx.Nonce = uint64(*dec.Nonce)
		if dec.To != nil {
			itx.To = dec.To
		}
		if dec.Gas == nil {
			return errors.New("missing required field 'gas' for txdata")
		}
		itx.Gas = uint64(*dec.Gas)
		if dec.MaxPriorityFeePerGas == nil {
			return errors.New("missing required field 'maxPriorityFeePerGas' for txdata")
		}
		itx.GasTipCap = (*big.Int)(dec.MaxPriorityFeePerGas)
		if dec.MaxFeePerGas == nil {
			return errors.New("missing required field 'maxFeePerGas' for txdata")
		}
		itx.GasFeeCap = (*big.Int)(dec.MaxFeePerGas)
		if dec.Value == nil {
			return errors.New("missing required field 'value' in transaction")
		}
		itx.Value = (*big.Int)(dec.Value)
		if dec.Input == nil {
			return errors.New("missing required field 'input' in transaction")
		}
		itx.Data = *dec.Input
		if dec.V == nil {
			return errors.New("missing required field 'v' in transaction")
		}
		if dec.AccessList != nil {
			itx.AccessList = *dec.AccessList
		}
		itx.V = (*big.Int)(dec.V)
		if dec.R == nil {
			return errors.New("missing required field 'r' in transaction")
		}
		itx.R = (*big.Int)(dec.R)
		if dec.S == nil {
			return errors.New("missing required field 's' in transaction")
		}
		itx.S = (*big.Int)(dec.S)
		withSignature := itx.V.Sign() != 0 || itx.R.Sign() != 0 || itx.S.Sign() != 0
		if withSignature {
			if err := sanityCheckSignature(itx.V, itx.R, itx.S, false); err != nil {
				return err
			}
		}

	case BlobTxType:
		var itx BlobTx
		inner = &itx
		if dec.ChainID == nil {
			return errors.New("missing required field 'chainId' in transaction")
		}
		itx.ChainID = uint256.MustFromBig((*big.Int)(dec.ChainID))
		if dec.Nonce == nil {
			return errors.New("missing required field 'nonce' in transaction")
		}
		itx.Nonce = uint64(*dec.Nonce)
		if dec.To != nil {
			itx.To = dec.To
		}
		if dec.Gas == nil {
			return errors.New("missing required field 'gas' for txdata")
		}
		itx.Gas = uint64(*dec.Gas)
		if dec.MaxPriorityFeePerGas == nil {
			return errors.New("missing required field 'maxPriorityFeePerGas' for txdata")
		}
		itx.GasTipCap = uint256.MustFromBig((*big.Int)(dec.MaxPriorityFeePerGas))
		if dec.MaxFeePerGas == nil {
			return errors.New("missing required field 'maxFeePerGas' for txdata")
		}
		itx.GasFeeCap = uint256.MustFromBig((*big.Int)(dec.MaxFeePerGas))
		if dec.MaxFeePerDataGas == nil {
			return errors.New("missing required field 'maxFeePerDataGas' for txdata")
		}
		itx.BlobFeeCap = uint256.MustFromBig((*big.Int)(dec.MaxFeePerDataGas))
		if dec.Value == nil {
			return errors.New("missing required field 'value' in transaction")
		}
		itx.Value = uint256.MustFromBig((*big.Int)(dec.Value))
		if dec.Input == nil {
			return errors.New("missing required field 'input' in transaction")
		}
		itx.Data = *dec.Input
		if dec.V == nil {
			return errors.New("missing required field 'v' in transaction")
		}
		if dec.AccessList != nil {
			itx.AccessList = *dec.AccessList
		}
		if dec.BlobVersionedHashes == nil {
			return errors.New("missing required field 'blobVersionedHashes' in transaction")
		}
		itx.BlobHashes = dec.BlobVersionedHashes
		itx.V = uint256.MustFromBig((*big.Int)(dec.V))
		if dec.R == nil {
			return errors.New("missing required field 'r' in transaction")
		}
		itx.R = uint256.MustFromBig((*big.Int)(dec.R))
		if dec.S == nil {
			return errors.New("missing required field 's' in transaction")
		}
		itx.S = uint256.MustFromBig((*big.Int)(dec.S))
		withSignature := itx.V.Sign() != 0 || itx.R.Sign() != 0 || itx.S.Sign() != 0
		if withSignature {
			if err := sanityCheckSignature(itx.V.ToBig(), itx.R.ToBig(), itx.S.ToBig(), false); err != nil {
				return err
			}
		}

	case DepositTxType:
		if dec.AccessList != nil || dec.MaxFeePerGas != nil ||
			dec.MaxPriorityFeePerGas != nil {
			return errors.New("unexpected field(s) in deposit transaction")
		}
		if dec.GasPrice != nil && dec.GasPrice.ToInt().Cmp(common.Big0) != 0 {
			return errors.New("deposit transaction GasPrice must be 0")
		}
		if (dec.V != nil && dec.V.ToInt().Cmp(common.Big0) != 0) ||
			(dec.R != nil && dec.R.ToInt().Cmp(common.Big0) != 0) ||
			(dec.S != nil && dec.S.ToInt().Cmp(common.Big0) != 0) {
			return errors.New("deposit transaction signature must be 0 or unset")
		}
		var itx DepositTx
		inner = &itx
		if dec.To != nil {
			itx.To = dec.To
		}
		if dec.Gas == nil {
			return errors.New("missing required field 'gas' for txdata")
		}
		itx.Gas = uint64(*dec.Gas)
		if dec.Value == nil {
			return errors.New("missing required field 'value' in transaction")
		}
		itx.Value = (*big.Int)(dec.Value)
		// mint may be omitted or nil if there is nothing to mint.
		itx.Mint = (*big.Int)(dec.Mint)
		if dec.Input == nil {
			return errors.New("missing required field 'input' in transaction")
		}
		itx.Data = *dec.Input
		if dec.From == nil {
			return errors.New("missing required field 'from' in transaction")
		}
		itx.From = *dec.From
		if dec.SourceHash == nil {
			return errors.New("missing required field 'sourceHash' in transaction")
		}
		itx.SourceHash = *dec.SourceHash
		// IsSystemTx may be omitted. Defaults to false.
		if dec.IsSystemTx != nil {
			itx.IsSystemTransaction = *dec.IsSystemTx
		}

		if dec.Nonce != nil {
			inner = &depositTxWithNonce{DepositTx: itx, EffectiveNonce: uint64(*dec.Nonce)}
		}
		log.Info("op-geth parsed DepositTransaction", "itx", itx)
	case OffchainTxType:
		if dec.AccessList != nil || dec.V != nil || dec.R != nil || dec.S != nil || dec.MaxFeePerGas != nil ||
			dec.MaxPriorityFeePerGas != nil || dec.GasPrice != nil || (dec.Nonce != nil && *dec.Nonce != 0) {
			return errors.New("unexpected field(s) in offchain transaction")
		}
		var itx OffchainTx
		inner = &itx
		if dec.To != nil {
			itx.To = dec.To
		}
		if dec.Gas == nil {
			return errors.New("missing required field 'gas' for txdata")
		}
		itx.Gas = uint64(*dec.Gas)
		if dec.Value == nil {
			return errors.New("missing required field 'value' in transaction")
		}
		itx.Value = (*big.Int)(dec.Value)
		// mint may be omitted or nil if there is nothing to mint.
		itx.Mint = (*big.Int)(dec.Mint)
		if dec.Data == nil {
			return errors.New("missing required field 'input' in transaction")
		}
		itx.Data = *dec.Data
		if dec.From == nil {
			return errors.New("missing required field 'from' in transaction")
		}
		itx.From = *dec.From
		if dec.SourceHash == nil {
			return errors.New("missing required field 'sourceHash' in transaction")
		}
		itx.SourceHash = *dec.SourceHash
		// IsSystemTx may be omitted. Defaults to false.
		if dec.IsSystemTx != nil {
			itx.IsSystemTransaction = *dec.IsSystemTx
		}
		// DEBUG level messages are not printed from op-geth when called by op-node
		log.Info("op-geth parsed OffchainTransaction", "itx", itx)
	default:
		return ErrTxTypeNotSupported
	}

	// Now set the inner transaction.
	tx.setDecoded(inner, 0)

	// TODO: check hash here?
	return nil
}

type depositTxWithNonce struct {
	DepositTx
	EffectiveNonce uint64
}

// EncodeRLP ensures that RLP encoding this transaction excludes the nonce. Otherwise, the tx Hash would change
func (tx *depositTxWithNonce) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, tx.DepositTx)
}

func (tx *depositTxWithNonce) effectiveNonce() *uint64 { return &tx.EffectiveNonce }
