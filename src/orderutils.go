// Package src provides utilities for generating and signing Polymarket CLOB orders.
// This is a vendored copy of github.com/polymarket/go-order-utils for security purposes.
// Source: https://github.com/Polymarket/go-order-utils (MIT License)
package src

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/rand"
	"errors"
	"fmt"
	"math"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// ==================== Model Types ====================

// OrderSignature is the order signature bytes
type OrderSignature = []byte

// OrderHash is the order hash
type OrderHash = common.Hash

// OrderBuilderSide represents the order side (BUY or SELL) for order building
type OrderBuilderSide = int

const (
	OrderBuilderBUY  OrderBuilderSide = 0
	OrderBuilderSELL OrderBuilderSide = 1
)

// SignatureType represents the type of signature
type SignatureType = int

const (
	// ECDSA EIP712 signatures signed by EOAs
	EOA SignatureType = iota
	// EIP712 signatures signed by EOAs that own Polymarket Proxy wallets
	POLY_PROXY
	// EIP712 signatures signed by EOAs that own Polymarket Gnosis safes
	POLY_GNOSIS_SAFE
)

// VerifyingContract represents which contract to verify against
type VerifyingContract = int

const (
	CTFExchange VerifyingContract = iota
	NegRiskCTFExchange
)

// OrderData is the input data for creating an order
type OrderData struct {
	Maker         string
	Taker         string
	TokenId       string
	MakerAmount   string
	TakerAmount   string
	FeeRateBps    string
	Nonce         string
	Signer        string
	Expiration    string
	Side          OrderBuilderSide
	SignatureType SignatureType
}

// Order is the structured order object
type Order struct {
	Salt          *big.Int
	TokenId       *big.Int
	MakerAmount   *big.Int
	TakerAmount   *big.Int
	Side          *big.Int
	Expiration    *big.Int
	Nonce         *big.Int
	FeeRateBps    *big.Int
	SignatureType *big.Int
	Maker         common.Address
	Taker         common.Address
	Signer        common.Address
}

// SignedOrder is an order with its signature
type SignedOrder struct {
	Order
	Signature OrderSignature
}

// ==================== Config ====================

// Contracts holds contract addresses for a chain
type Contracts struct {
	Exchange         common.Address
	FeeModule        common.Address
	NegRiskExchange  common.Address
	NegRiskFeeModule common.Address
	NegRiskAdapter   common.Address
	Collateral       common.Address
	Conditional      common.Address
}

var (
	_MATIC_CONTRACTS = &Contracts{
		Exchange:         common.HexToAddress("0x4bFb41d5B3570DeFd03C39a9A4D8dE6Bd8B8982E"),
		FeeModule:        common.HexToAddress("0xE3f18aCc55091e2c48d883fc8C8413319d4Ab7b0"),
		NegRiskExchange:  common.HexToAddress("0xC5d563A36AE78145C45a50134d48A1215220f80a"),
		NegRiskFeeModule: common.HexToAddress("0xB768891e3130F6dF18214Ac804d4DB76c2C37730"),
		NegRiskAdapter:   common.HexToAddress("0xd91E80cF2E7be2e162c6513ceD06f1dD0dA35296"),
		Collateral:       common.HexToAddress("0x2791Bca1f2de4661ED88A30C99A7a9449Aa84174"),
		Conditional:      common.HexToAddress("0x4D97DCd97eC945f40cF65F87097ACe5EA0476045"),
	}

	_AMOY_CONTRACTS = &Contracts{
		Exchange:         common.HexToAddress("0xdFE02Eb6733538f8Ea35D585af8DE5958AD99E40"),
		FeeModule:        common.HexToAddress("0xE3f18aCc55091e2c48d883fc8C8413319d4Ab7b0"),
		NegRiskExchange:  common.HexToAddress("0xC5d563A36AE78145C45a50134d48A1215220f80a"),
		NegRiskFeeModule: common.HexToAddress("0xB768891e3130F6dF18214Ac804d4DB76c2C37730"),
		NegRiskAdapter:   common.HexToAddress("0xd91E80cF2E7be2e162c6513ceD06f1dD0dA35296"),
		Collateral:       common.HexToAddress("0x9c4e1703476e875070ee25b56a58b008cfb8fa78"),
		Conditional:      common.HexToAddress("0x69308FB512518e39F9b16112fA8d994F4e2Bf8bB"),
	}
)

// GetContracts returns contracts for a chain ID
func GetContracts(chainId int64) (*Contracts, error) {
	switch chainId {
	case 137:
		return _MATIC_CONTRACTS, nil
	case 80002:
		return _AMOY_CONTRACTS, nil
	default:
		return nil, fmt.Errorf("invalid chain id: %d", chainId)
	}
}

// ==================== ABI Types ====================
// Types solidity=>go types from go-ethereum

var (
	abiUint256, _ = abi.NewType("uint256", "", nil)
	abiUint8, _   = abi.NewType("uint8", "", nil)
	abiBytes32, _ = abi.NewType("bytes32", "", nil)
	abiAddress, _ = abi.NewType("address", "", nil)
)

// ==================== EIP712 Constants ====================

var (
	// Pre-computed keccak256 hashes of name and version (as the official lib does)
	_PROTOCOL_NAME    = crypto.Keccak256Hash([]byte("Polymarket CTF Exchange"))
	_PROTOCOL_VERSION = crypto.Keccak256Hash([]byte("1"))

	// EIP712Domain type hash
	_EIP712_DOMAIN_HASH = crypto.Keccak256Hash(
		[]byte("EIP712Domain(string name,string version,uint256 chainId,address verifyingContract)"),
	)

	// Order type hash - must match exactly what the contract expects
	_ORDER_STRUCTURE_HASH = crypto.Keccak256Hash(
		[]byte("Order(uint256 salt,address maker,address signer,address taker,uint256 tokenId,uint256 makerAmount,uint256 takerAmount,uint256 expiration,uint256 nonce,uint256 feeRateBps,uint8 side,uint8 signatureType)"),
	)
)

// EIP712 domain separator types for abi.Pack
var _EIP712_DOMAIN_TYPES = []abi.Type{
	abiBytes32, // typehash
	abiBytes32, // name (keccak256)
	abiBytes32, // version (keccak256)
	abiUint256, // chainId
	abiAddress, // verifyingContract
}

// Order structure types for abi.Pack
var _ORDER_STRUCTURE_TYPES = []abi.Type{
	abiBytes32, // typehash
	abiUint256, // salt
	abiAddress, // maker
	abiAddress, // signer
	abiAddress, // taker
	abiUint256, // tokenId
	abiUint256, // makerAmount
	abiUint256, // takerAmount
	abiUint256, // expiration
	abiUint256, // nonce
	abiUint256, // feeRateBps
	abiUint8,   // side
	abiUint8,   // signatureType
}

// ==================== ABI Encoding ====================

// abiEncode uses go-ethereum's abi.Pack for proper ABI encoding
func abiEncode(types []abi.Type, values []interface{}) ([]byte, error) {
	arguments := make([]abi.Argument, 0, len(types))
	for _, t := range types {
		arguments = append(arguments, abi.Argument{Type: t})
	}
	return abi.Arguments(arguments).Pack(values...)
}

// ==================== EIP712 Functions ====================

// buildEIP712DomainSeparator creates the domain separator using abi.Pack
func buildEIP712DomainSeparator(name, version common.Hash, chainId *big.Int, address common.Address) (common.Hash, error) {
	values := []interface{}{
		_EIP712_DOMAIN_HASH,
		name,
		version,
		chainId,
		address,
	}

	encoded, err := abiEncode(_EIP712_DOMAIN_TYPES, values)
	if err != nil {
		return common.Hash{}, err
	}

	domainSeparator := crypto.Keccak256Hash(encoded)

	// DEBUG: Format matching Python output
	fmt.Println("\n======================================================================")
	fmt.Println("[EIP-712 Hash Calculation - Go]")
	fmt.Println("======================================================================")
	fmt.Printf("\n1. EIP712_DOMAIN_HASH: %s\n", _EIP712_DOMAIN_HASH.Hex()[2:])
	fmt.Printf("2. Name Hash:          %s\n", name.Hex()[2:])
	fmt.Printf("3. Version Hash:       %s\n", version.Hex()[2:])
	fmt.Printf("4. Domain Separator:   %s\n", domainSeparator.Hex()[2:])

	return domainSeparator, nil
}

// hashTypedDataV4 creates the EIP-712 hash of the order
func hashTypedDataV4(domainSeparator common.Hash, order *Order) (common.Hash, error) {
	values := []interface{}{
		_ORDER_STRUCTURE_HASH,
		order.Salt,
		order.Maker,
		order.Signer,
		order.Taker,
		order.TokenId,
		order.MakerAmount,
		order.TakerAmount,
		order.Expiration,
		order.Nonce,
		order.FeeRateBps,
		uint8(order.Side.Uint64()),          // side as uint8
		uint8(order.SignatureType.Uint64()), // signatureType as uint8
	}

	encoded, err := abiEncode(_ORDER_STRUCTURE_TYPES, values)
	if err != nil {
		return common.Hash{}, err
	}

	orderStructHash := crypto.Keccak256Hash(encoded)

	fmt.Printf("5. ORDER_TYPE_HASH:    %s\n", _ORDER_STRUCTURE_HASH.Hex()[2:])
	fmt.Printf("6. Order Struct Hash:  %s\n", orderStructHash.Hex()[2:])

	// EIP-712 final hash: keccak256("\x19\x01" ‖ domainSeparator ‖ structHash)
	rawData := make([]byte, 0, 66) // 2 + 32 + 32
	rawData = append(rawData, 0x19, 0x01)
	rawData = append(rawData, domainSeparator[:]...)
	rawData = append(rawData, orderStructHash[:]...)

	finalHash := crypto.Keccak256Hash(rawData)

	fmt.Printf("7. Final Message Hash: %s\n", finalHash.Hex()[2:])
	fmt.Println("======================================================================")

	return finalHash, nil
}

// ==================== Signer Functions ====================

var ErrInvalidSignatureLen = errors.New("invalid signature length")

// signOrderHash signs the order hash with the private key
// IMPORTANT: V byte must be 27 or 28 per EIP-155
func signOrderHash(privateKey *ecdsa.PrivateKey, hashedData common.Hash) ([]byte, error) {
	sign, err := crypto.Sign(hashedData.Bytes(), privateKey)
	if err != nil {
		return nil, err
	}
	sign[64] += 27 // EIP-155: Transform V from 0/1 to 27/28
	return sign, nil
}

// validateSignature verifies that the signature matches the signer
func validateSignature(signer common.Address, hashedData common.Hash, signature []byte) (bool, error) {
	sigCopy := make([]byte, len(signature))
	copy(sigCopy, signature)

	if len(sigCopy) != 65 {
		return false, ErrInvalidSignatureLen
	}

	// Transform V from 27/28 back to 0/1 for ecrecover
	if sigCopy[64] != 0 && sigCopy[64] != 1 {
		sigCopy[64] -= 27
	}

	sigPublicKey, err := crypto.Ecrecover(hashedData.Bytes(), sigCopy)
	if err != nil {
		return false, err
	}

	recoveredPublicKey, err := crypto.UnmarshalPubkey(sigPublicKey)
	if err != nil {
		return false, err
	}

	recoveredAddress := crypto.PubkeyToAddress(*recoveredPublicKey)
	return bytes.Equal(signer.Bytes(), recoveredAddress.Bytes()), nil
}

// ==================== Utils Functions ====================

func generateRandomSalt() int64 {
	maxInt := math.Pow(2, 32)
	nBig, _ := rand.Int(rand.Reader, big.NewInt(int64(maxInt)))
	return nBig.Int64()
}

func getVerifyingContractAddress(chainId *big.Int, contract VerifyingContract) (common.Address, error) {
	contracts, err := GetContracts(chainId.Int64())
	if err != nil {
		return common.Address{}, err
	}

	switch contract {
	case CTFExchange:
		return contracts.Exchange, nil
	case NegRiskCTFExchange:
		return contracts.NegRiskExchange, nil
	}

	return common.Address{}, fmt.Errorf("invalid contract")
}

// ==================== Order Builder ====================

// ExchangeOrderBuilder builds and signs orders for Polymarket CLOB
type ExchangeOrderBuilder struct {
	chainId       *big.Int
	saltGenerator func() int64
}

// NewExchangeOrderBuilder creates a new order builder for the given chain
func NewExchangeOrderBuilder(chainId *big.Int, saltGenerator func() int64) *ExchangeOrderBuilder {
	if saltGenerator == nil {
		saltGenerator = generateRandomSalt
	}
	return &ExchangeOrderBuilder{
		chainId:       chainId,
		saltGenerator: saltGenerator,
	}
}

// BuildSignedOrder builds an order and signs it
func (e *ExchangeOrderBuilder) BuildSignedOrder(privateKey *ecdsa.PrivateKey, orderData *OrderData, contract VerifyingContract) (*SignedOrder, error) {
	order, err := e.BuildOrder(orderData)
	if err != nil {
		return nil, err
	}

	orderHash, err := e.BuildOrderHash(order, contract)
	if err != nil {
		return nil, err
	}

	signature, err := e.BuildOrderSignature(privateKey, orderHash)
	if err != nil {
		return nil, err
	}

	ok, err := validateSignature(order.Signer, orderHash, signature)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("signature error")
	}

	return &SignedOrder{
		Order:     *order,
		Signature: signature,
	}, nil
}

// BuildOrder creates an Order object from order data
func (e *ExchangeOrderBuilder) BuildOrder(orderData *OrderData) (*Order, error) {
	var signer common.Address
	if orderData.Signer == "" {
		signer = common.HexToAddress(orderData.Maker)
	} else {
		signer = common.HexToAddress(orderData.Signer)
	}

	var tokenId *big.Int
	var ok bool
	if tokenId, ok = new(big.Int).SetString(orderData.TokenId, 10); !ok {
		return nil, fmt.Errorf("can't parse TokenId: %s as valid *big.Int", orderData.TokenId)
	}

	var makerAmount *big.Int
	if makerAmount, ok = new(big.Int).SetString(orderData.MakerAmount, 10); !ok {
		return nil, fmt.Errorf("can't parse MakerAmount: %s as valid *big.Int", orderData.MakerAmount)
	}

	var takerAmount *big.Int
	if takerAmount, ok = new(big.Int).SetString(orderData.TakerAmount, 10); !ok {
		return nil, fmt.Errorf("can't parse TakerAmount: %s as valid *big.Int", orderData.TakerAmount)
	}

	var expiration *big.Int
	if orderData.Expiration == "" {
		orderData.Expiration = "0"
	}
	if expiration, ok = new(big.Int).SetString(orderData.Expiration, 10); !ok {
		return nil, fmt.Errorf("can't parse Expiration: %s as valid *big.Int", orderData.Expiration)
	}

	var nonce *big.Int
	if nonce, ok = new(big.Int).SetString(orderData.Nonce, 10); !ok {
		return nil, fmt.Errorf("can't parse Nonce: %s as valid *big.Int", orderData.Nonce)
	}

	var feeRateBps *big.Int
	if feeRateBps, ok = new(big.Int).SetString(orderData.FeeRateBps, 10); !ok {
		return nil, fmt.Errorf("can't parse FeeRateBps: %s as valid *big.Int", orderData.FeeRateBps)
	}

	return &Order{
		Salt:          new(big.Int).SetInt64(e.saltGenerator()),
		Maker:         common.HexToAddress(orderData.Maker),
		Taker:         common.HexToAddress(orderData.Taker),
		Signer:        signer,
		TokenId:       tokenId,
		MakerAmount:   makerAmount,
		TakerAmount:   takerAmount,
		Side:          new(big.Int).SetInt64(int64(orderData.Side)),
		Expiration:    expiration,
		Nonce:         nonce,
		FeeRateBps:    feeRateBps,
		SignatureType: new(big.Int).SetInt64(int64(orderData.SignatureType)),
	}, nil
}

// BuildOrderHash generates the EIP712 hash of the order
func (e *ExchangeOrderBuilder) BuildOrderHash(order *Order, contract VerifyingContract) (OrderHash, error) {
	verifyingContract, err := getVerifyingContractAddress(e.chainId, contract)
	if err != nil {
		return OrderHash{}, err
	}

	domainSeparator, err := buildEIP712DomainSeparator(_PROTOCOL_NAME, _PROTOCOL_VERSION, e.chainId, verifyingContract)
	if err != nil {
		return OrderHash{}, err
	}

	orderHash, err := hashTypedDataV4(domainSeparator, order)
	if err != nil {
		return OrderHash{}, err
	}

	return orderHash, nil
}

// BuildOrderSignature signs the order hash
func (e *ExchangeOrderBuilder) BuildOrderSignature(privateKey *ecdsa.PrivateKey, orderHash OrderHash) (OrderSignature, error) {
	return signOrderHash(privateKey, orderHash)
}
