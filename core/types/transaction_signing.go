package types

import (
	"crypto/ecdsa"
	"errors"
	"fmt"
	"math/big"

	"github.com/siotchain/siot/helper"
	"github.com/siotchain/siot/crypto"
	"github.com/siotchain/siot/params"
)

var ErrInvalidChainId = errors.New("invalid chaid id for signer")

// sigCache is used to cache the derived sender and contains
// the signer used to derive it.
type sigCache struct {
	signer Signer
	from   helper.Address
}

// MakeSigner returns a Signer based on the given chain config and block number.
func MakeSigner(config *params.ChainConfig, blockNumber *big.Int) Signer {
	var signer Signer
	switch {
	case config.IsEIP155(blockNumber):
		signer = NewEIP155Signer(config.ChainId)
	case config.IsHomestead(blockNumber):
		signer = HomesteadSigner{}
	default:
		signer = FrontierSigner{}
	}
	return signer
}

// SignECDSA signs the transaction using the given signer and private key
func SignECDSA(s Signer, tx *Transaction, prv *ecdsa.PrivateKey) (*Transaction, error) {
	h := s.Hash(tx)
	sig, err := crypto.SignSiotchain(h[:], prv)
	if err != nil {
		return nil, err
	}
	return s.WithSignature(tx, sig)
}

// Sender derives the sender from the tx using the signer derivation
// functions.

// Sender returns the address derived from the signature (V, R, S) using secp256k1
// elliptic curve and an error if it failed deriving or upon an incorrect
// signature.
//
// Sender may cache the address, allowing it to be used regardless of
// signing method. The cache is invalidated if the cached signer does
// not match the signer used in the current call.
func Sender(signer Signer, tx *Transaction) (helper.Address, error) {
	if sc := tx.from.Load(); sc != nil {
		sigCache := sc.(sigCache)
		// If the signer used to derive from in a previous
		// call is not the same as used current, invalidate
		// the cache.
		if sigCache.signer.Equal(signer) {
			return sigCache.from, nil
		}
	}

	pubkey, err := signer.PublicKey(tx)
	if err != nil {
		return helper.Address{}, err
	}
	var addr helper.Address
	copy(addr[:], crypto.Keccak256(pubkey[1:])[12:])
	tx.from.Store(sigCache{signer: signer, from: addr})
	return addr, nil
}

// SignatureValues returns the ECDSA signature values contained in the transaction.
func SignatureValues(signer Signer, tx *Transaction) (v byte, r *big.Int, s *big.Int) {
	return normaliseV(signer, tx.data.V), new(big.Int).Set(tx.data.R), new(big.Int).Set(tx.data.S)
}

type Signer interface {
	// Hash returns the rlp encoded hash for signatures
	Hash(tx *Transaction) helper.Hash
	// PubilcKey returns the public key derived from the signature
	PublicKey(tx *Transaction) ([]byte, error)
	// SignECDSA signs the transaction with the given and returns a copy of the tx
	SignECDSA(tx *Transaction, prv *ecdsa.PrivateKey) (*Transaction, error)
	// WithSignature returns a copy of the transaction with the given signature
	WithSignature(tx *Transaction, sig []byte) (*Transaction, error)
	// Checks for equality on the signers
	Equal(Signer) bool
}

// EIP155Transaction implements TransactionInterface using the
// EIP155 rules
type EIP155Signer struct {
	HomesteadSigner

	chainId, chainIdMul *big.Int
}

func NewEIP155Signer(chainId *big.Int) EIP155Signer {
	return EIP155Signer{
		chainId:    chainId,
		chainIdMul: new(big.Int).Mul(chainId, big.NewInt(2)),
	}
}

func (s EIP155Signer) Equal(s2 Signer) bool {
	eip155, ok := s2.(EIP155Signer)
	return ok && eip155.chainId.Cmp(s.chainId) == 0
}

func (s EIP155Signer) SignECDSA(tx *Transaction, prv *ecdsa.PrivateKey) (*Transaction, error) {
	return SignECDSA(s, tx, prv)
}

func (s EIP155Signer) PublicKey(tx *Transaction) ([]byte, error) {
	// if the transaction is not protected fall back to homestead signer
	if !tx.Protected() {
		return (HomesteadSigner{}).PublicKey(tx)
	}

	if tx.ChainId().Cmp(s.chainId) != 0 {
		return nil, ErrInvalidChainId
	}

	V := normaliseV(s, tx.data.V)
	if !crypto.ValidateSignatureValues(V, tx.data.R, tx.data.S, true) {
		return nil, ErrInvalidSig
	}

	// encode the signature in uncompressed format
	R, S := tx.data.R.Bytes(), tx.data.S.Bytes()
	sig := make([]byte, 65)
	copy(sig[32-len(R):32], R)
	copy(sig[64-len(S):64], S)
	sig[64] = V - 27

	// recover the public key from the signature
	hash := s.Hash(tx)
	pub, err := crypto.Ecrecover(hash[:], sig)
	if err != nil {
		return nil, err
	}
	if len(pub) == 0 || pub[0] != 4 {
		return nil, errors.New("invalid public key")
	}
	return pub, nil
}

// WithSignature returns a new transaction with the given signature.
// This signature needs to be formatted as described in the yellow paper (v+27).
func (s EIP155Signer) WithSignature(tx *Transaction, sig []byte) (*Transaction, error) {
	if len(sig) != 65 {
		panic(fmt.Sprintf("wrong size for snature: got %d, want 65", len(sig)))
	}

	cpy := &Transaction{data: tx.data}
	cpy.data.R = new(big.Int).SetBytes(sig[:32])
	cpy.data.S = new(big.Int).SetBytes(sig[32:64])
	cpy.data.V = new(big.Int).SetBytes([]byte{sig[64]})
	if s.chainId.BitLen() > 0 {
		cpy.data.V = big.NewInt(int64(sig[64] - 27 + 35))
		cpy.data.V.Add(cpy.data.V, s.chainIdMul)
	}
	return cpy, nil
}

// Hash returns the hash to be signed by the sender.
// It does not uniquely identify the transaction.
func (s EIP155Signer) Hash(tx *Transaction) helper.Hash {
	return rlpHash([]interface{}{
		tx.data.AccountNonce,
		tx.data.Price,
		tx.data.GasLimit,
		tx.data.Recipient,
		tx.data.Amount,
		tx.data.Payload,
		s.chainId, uint(0), uint(0),
	})
}

func (s EIP155Signer) SigECDSA(tx *Transaction, prv *ecdsa.PrivateKey) (*Transaction, error) {
	h := s.Hash(tx)
	sig, err := crypto.SignSiotchain(h[:], prv)
	if err != nil {
		return nil, err
	}
	return s.WithSignature(tx, sig)
}

// HomesteadTransaction implements TransactionInterface using the
// homestead rules.
type HomesteadSigner struct{ FrontierSigner }

func (s HomesteadSigner) Equal(s2 Signer) bool {
	_, ok := s2.(HomesteadSigner)
	return ok
}

// WithSignature returns a new transaction with the given snature.
// This snature needs to be formatted as described in the yellow paper (v+27).
func (hs HomesteadSigner) WithSignature(tx *Transaction, sig []byte) (*Transaction, error) {
	if len(sig) != 65 {
		panic(fmt.Sprintf("wrong size for snature: got %d, want 65", len(sig)))
	}
	cpy := &Transaction{data: tx.data}
	cpy.data.R = new(big.Int).SetBytes(sig[:32])
	cpy.data.S = new(big.Int).SetBytes(sig[32:64])
	cpy.data.V = new(big.Int).SetBytes([]byte{sig[64]})
	return cpy, nil
}

func (hs HomesteadSigner) SignECDSA(tx *Transaction, prv *ecdsa.PrivateKey) (*Transaction, error) {
	h := hs.Hash(tx)
	sig, err := crypto.SignSiotchain(h[:], prv)
	if err != nil {
		return nil, err
	}
	return hs.WithSignature(tx, sig)
}

func (hs HomesteadSigner) PublicKey(tx *Transaction) ([]byte, error) {
	if tx.data.V.BitLen() > 8 {
		return nil, ErrInvalidSig
	}
	V := byte(tx.data.V.Uint64())
	if !crypto.ValidateSignatureValues(V, tx.data.R, tx.data.S, true) {
		return nil, ErrInvalidSig
	}
	// encode the snature in uncompressed format
	r, s := tx.data.R.Bytes(), tx.data.S.Bytes()
	sig := make([]byte, 65)
	copy(sig[32-len(r):32], r)
	copy(sig[64-len(s):64], s)
	sig[64] = V - 27

	// recover the public key from the snature
	hash := hs.Hash(tx)
	pub, err := crypto.Ecrecover(hash[:], sig)
	if err != nil {
		return nil, err
	}
	if len(pub) == 0 || pub[0] != 4 {
		return nil, errors.New("invalid public key")
	}
	return pub, nil
}

type FrontierSigner struct{}

func (s FrontierSigner) Equal(s2 Signer) bool {
	_, ok := s2.(FrontierSigner)
	return ok
}

// WithSignature returns a new transaction with the given snature.
// This snature needs to be formatted as described in the yellow paper (v+27).
func (fs FrontierSigner) WithSignature(tx *Transaction, sig []byte) (*Transaction, error) {
	if len(sig) != 65 {
		panic(fmt.Sprintf("wrong size for snature: got %d, want 65", len(sig)))
	}
	cpy := &Transaction{data: tx.data}
	cpy.data.R = new(big.Int).SetBytes(sig[:32])
	cpy.data.S = new(big.Int).SetBytes(sig[32:64])
	cpy.data.V = new(big.Int).SetBytes([]byte{sig[64]})
	return cpy, nil
}

func (fs FrontierSigner) SignECDSA(tx *Transaction, prv *ecdsa.PrivateKey) (*Transaction, error) {
	h := fs.Hash(tx)
	sig, err := crypto.SignSiotchain(h[:], prv)
	if err != nil {
		return nil, err
	}
	return fs.WithSignature(tx, sig)
}

// Hash returns the hash to be sned by the sender.
// It does not uniquely identify the transaction.
func (fs FrontierSigner) Hash(tx *Transaction) helper.Hash {
	return rlpHash([]interface{}{
		tx.data.AccountNonce,
		tx.data.Price,
		tx.data.GasLimit,
		tx.data.Recipient,
		tx.data.Amount,
		tx.data.Payload,
	})
}

func (fs FrontierSigner) PublicKey(tx *Transaction) ([]byte, error) {
	if tx.data.V.BitLen() > 8 {
		return nil, ErrInvalidSig
	}

	V := byte(tx.data.V.Uint64())
	if !crypto.ValidateSignatureValues(V, tx.data.R, tx.data.S, false) {
		return nil, ErrInvalidSig
	}
	// encode the snature in uncompressed format
	r, s := tx.data.R.Bytes(), tx.data.S.Bytes()
	sig := make([]byte, 65)
	copy(sig[32-len(r):32], r)
	copy(sig[64-len(s):64], s)
	sig[64] = V - 27

	// recover the public key from the snature
	hash := fs.Hash(tx)
	pub, err := crypto.Ecrecover(hash[:], sig)
	if err != nil {
		return nil, err
	}
	if len(pub) == 0 || pub[0] != 4 {
		return nil, errors.New("invalid public key")
	}
	return pub, nil
}

// normaliseV returns the Siotchain version of the V parameter
func normaliseV(s Signer, v *big.Int) byte {
	if s, ok := s.(EIP155Signer); ok {
		stdV := v.BitLen() <= 8 && (v.Uint64() == 27 || v.Uint64() == 28)
		if s.chainId.BitLen() > 0 && !stdV {
			nv := byte((new(big.Int).Sub(v, s.chainIdMul).Uint64()) - 35 + 27)
			return nv
		}
	}
	return byte(v.Uint64())
}

// deriveChainId derives the chain id from the given v parameter
func deriveChainId(v *big.Int) *big.Int {
	if v.BitLen() <= 64 {
		v := v.Uint64()
		if v == 27 || v == 28 {
			return new(big.Int)
		}
		return new(big.Int).SetUint64((v - 35) / 2)
	}
	v = new(big.Int).Sub(v, big.NewInt(35))
	return v.Div(v, big.NewInt(2))
}
