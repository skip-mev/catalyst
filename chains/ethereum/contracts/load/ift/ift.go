// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package ift

import (
	"errors"
	"math/big"
	"strings"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
)

// Reference imports to suppress errors if they are not otherwise used.
var (
	_ = errors.New
	_ = big.NewInt
	_ = strings.NewReader
	_ = ethereum.NotFound
	_ = bind.Bind
	_ = common.Big1
	_ = types.BloomLookup
	_ = event.NewSubscription
	_ = abi.ConvertType
)

// IftMetaData contains all meta data concerning the Ift contract.
var IftMetaData = &bind.MetaData{
	ABI: "[{\"type\":\"function\",\"name\":\"iftTransfer\",\"inputs\":[{\"name\":\"clientId\",\"type\":\"string\",\"internalType\":\"string\"},{\"name\":\"receiver\",\"type\":\"string\",\"internalType\":\"string\"},{\"name\":\"amount\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"timeoutTimestamp\",\"type\":\"uint64\",\"internalType\":\"uint64\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"}]",
	Bin: "0x6080604052348015600f57600080fd5b506101658061001f6000396000f3fe608060405234801561001057600080fd5b506004361061002b5760003560e01c8063711708b314610030575b600080fd5b61004661003e366004610091565b505050505050565b005b60008083601f84011261005a57600080fd5b50813567ffffffffffffffff81111561007257600080fd5b60208301915083602082850101111561008a57600080fd5b9250929050565b600080600080600080608087890312156100aa57600080fd5b863567ffffffffffffffff8111156100c157600080fd5b6100cd89828a01610048565b909750955050602087013567ffffffffffffffff8111156100ed57600080fd5b6100f989828a01610048565b90955093505060408701359150606087013567ffffffffffffffff8116811461012157600080fd5b80915050929550929550929556fea2646970667358221220d8a942edfee86403f43ae2807e0d0f7e3b122e7521ad6df026ce5d632ca7ccaa64736f6c634300081c0033",
}

// IftABI is the input ABI used to generate the binding from.
// Deprecated: Use IftMetaData.ABI instead.
var IftABI = IftMetaData.ABI

// IftBin is the compiled bytecode used for deploying new contracts.
// Deprecated: Use IftMetaData.Bin instead.
var IftBin = IftMetaData.Bin

// DeployIft deploys a new Ethereum contract, binding an instance of Ift to it.
func DeployIft(auth *bind.TransactOpts, backend bind.ContractBackend) (common.Address, *types.Transaction, *Ift, error) {
	parsed, err := IftMetaData.GetAbi()
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	if parsed == nil {
		return common.Address{}, nil, nil, errors.New("GetABI returned nil")
	}

	address, tx, contract, err := bind.DeployContract(auth, *parsed, common.FromHex(IftBin), backend)
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	return address, tx, &Ift{IftCaller: IftCaller{contract: contract}, IftTransactor: IftTransactor{contract: contract}, IftFilterer: IftFilterer{contract: contract}}, nil
}

// Ift is an auto generated Go binding around an Ethereum contract.
type Ift struct {
	IftCaller     // Read-only binding to the contract
	IftTransactor // Write-only binding to the contract
	IftFilterer   // Log filterer for contract events
}

// IftCaller is an auto generated read-only Go binding around an Ethereum contract.
type IftCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// IftTransactor is an auto generated write-only Go binding around an Ethereum contract.
type IftTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// IftFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type IftFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// IftSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type IftSession struct {
	Contract     *Ift              // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// IftCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type IftCallerSession struct {
	Contract *IftCaller    // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts // Call options to use throughout this session
}

// IftTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type IftTransactorSession struct {
	Contract     *IftTransactor    // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// IftRaw is an auto generated low-level Go binding around an Ethereum contract.
type IftRaw struct {
	Contract *Ift // Generic contract binding to access the raw methods on
}

// IftCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type IftCallerRaw struct {
	Contract *IftCaller // Generic read-only contract binding to access the raw methods on
}

// IftTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type IftTransactorRaw struct {
	Contract *IftTransactor // Generic write-only contract binding to access the raw methods on
}

// NewIft creates a new instance of Ift, bound to a specific deployed contract.
func NewIft(address common.Address, backend bind.ContractBackend) (*Ift, error) {
	contract, err := bindIft(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &Ift{IftCaller: IftCaller{contract: contract}, IftTransactor: IftTransactor{contract: contract}, IftFilterer: IftFilterer{contract: contract}}, nil
}

// NewIftCaller creates a new read-only instance of Ift, bound to a specific deployed contract.
func NewIftCaller(address common.Address, caller bind.ContractCaller) (*IftCaller, error) {
	contract, err := bindIft(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &IftCaller{contract: contract}, nil
}

// NewIftTransactor creates a new write-only instance of Ift, bound to a specific deployed contract.
func NewIftTransactor(address common.Address, transactor bind.ContractTransactor) (*IftTransactor, error) {
	contract, err := bindIft(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &IftTransactor{contract: contract}, nil
}

// NewIftFilterer creates a new log filterer instance of Ift, bound to a specific deployed contract.
func NewIftFilterer(address common.Address, filterer bind.ContractFilterer) (*IftFilterer, error) {
	contract, err := bindIft(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &IftFilterer{contract: contract}, nil
}

// bindIft binds a generic wrapper to an already deployed contract.
func bindIft(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := IftMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Ift *IftRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _Ift.Contract.IftCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Ift *IftRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Ift.Contract.IftTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Ift *IftRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Ift.Contract.IftTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Ift *IftCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _Ift.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Ift *IftTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Ift.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Ift *IftTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Ift.Contract.contract.Transact(opts, method, params...)
}

// IftTransfer is a paid mutator transaction binding the contract method 0x711708b3.
//
// Solidity: function iftTransfer(string clientId, string receiver, uint256 amount, uint64 timeoutTimestamp) returns()
func (_Ift *IftTransactor) IftTransfer(opts *bind.TransactOpts, clientId string, receiver string, amount *big.Int, timeoutTimestamp uint64) (*types.Transaction, error) {
	return _Ift.contract.Transact(opts, "iftTransfer", clientId, receiver, amount, timeoutTimestamp)
}

// IftTransfer is a paid mutator transaction binding the contract method 0x711708b3.
//
// Solidity: function iftTransfer(string clientId, string receiver, uint256 amount, uint64 timeoutTimestamp) returns()
func (_Ift *IftSession) IftTransfer(clientId string, receiver string, amount *big.Int, timeoutTimestamp uint64) (*types.Transaction, error) {
	return _Ift.Contract.IftTransfer(&_Ift.TransactOpts, clientId, receiver, amount, timeoutTimestamp)
}

// IftTransfer is a paid mutator transaction binding the contract method 0x711708b3.
//
// Solidity: function iftTransfer(string clientId, string receiver, uint256 amount, uint64 timeoutTimestamp) returns()
func (_Ift *IftTransactorSession) IftTransfer(clientId string, receiver string, amount *big.Int, timeoutTimestamp uint64) (*types.Transaction, error) {
	return _Ift.Contract.IftTransfer(&_Ift.TransactOpts, clientId, receiver, amount, timeoutTimestamp)
}
