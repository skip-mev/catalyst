// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package target

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

// TargetMetaData contains all meta data concerning the Target contract.
var TargetMetaData = &bind.MetaData{
	ABI: "[{\"type\":\"function\",\"name\":\"data\",\"inputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"process\",\"inputs\":[{\"name\":\"value\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"pure\"},{\"type\":\"function\",\"name\":\"store\",\"inputs\":[{\"name\":\"key\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"value\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"}]",
	Bin: "0x6080604052348015600e575f5ffd5b5061026d8061001c5f395ff3fe608060405234801561000f575f5ffd5b506004361061003f575f3560e01c80636ed28ed014610043578063f0ba84401461005f578063ffb2c4791461008f575b5f5ffd5b61005d60048036038101906100589190610138565b6100bf565b005b61007960048036038101906100749190610176565b6100d8565b60405161008691906101b0565b60405180910390f35b6100a960048036038101906100a49190610176565b6100ec565b6040516100b691906101b0565b60405180910390f35b805f5f8481526020019081526020015f20819055505050565b5f602052805f5260405f205f915090505481565b5f6002826100fa91906101f6565b9050919050565b5f5ffd5b5f819050919050565b61011781610105565b8114610121575f5ffd5b50565b5f813590506101328161010e565b92915050565b5f5f6040838503121561014e5761014d610101565b5b5f61015b85828601610124565b925050602061016c85828601610124565b9150509250929050565b5f6020828403121561018b5761018a610101565b5b5f61019884828501610124565b91505092915050565b6101aa81610105565b82525050565b5f6020820190506101c35f8301846101a1565b92915050565b7f4e487b71000000000000000000000000000000000000000000000000000000005f52601160045260245ffd5b5f61020082610105565b915061020b83610105565b925082820261021981610105565b915082820484148315176102305761022f6101c9565b5b509291505056fea26469706673582212203ae612d896093dca62254cc28ed21c31820f7ad45ff5b003c64248ab96b1200264736f6c634300081e0033",
}

// TargetABI is the input ABI used to generate the binding from.
// Deprecated: Use TargetMetaData.ABI instead.
var TargetABI = TargetMetaData.ABI

// TargetBin is the compiled bytecode used for deploying new contracts.
// Deprecated: Use TargetMetaData.Bin instead.
var TargetBin = TargetMetaData.Bin

// DeployTarget deploys a new Ethereum contract, binding an instance of Target to it.
func DeployTarget(auth *bind.TransactOpts, backend bind.ContractBackend) (common.Address, *types.Transaction, *Target, error) {
	parsed, err := TargetMetaData.GetAbi()
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	if parsed == nil {
		return common.Address{}, nil, nil, errors.New("GetABI returned nil")
	}

	address, tx, contract, err := bind.DeployContract(auth, *parsed, common.FromHex(TargetBin), backend)
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	return address, tx, &Target{TargetCaller: TargetCaller{contract: contract}, TargetTransactor: TargetTransactor{contract: contract}, TargetFilterer: TargetFilterer{contract: contract}}, nil
}

// Target is an auto generated Go binding around an Ethereum contract.
type Target struct {
	TargetCaller     // Read-only binding to the contract
	TargetTransactor // Write-only binding to the contract
	TargetFilterer   // Log filterer for contract events
}

// TargetCaller is an auto generated read-only Go binding around an Ethereum contract.
type TargetCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// TargetTransactor is an auto generated write-only Go binding around an Ethereum contract.
type TargetTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// TargetFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type TargetFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// TargetSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type TargetSession struct {
	Contract     *Target           // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// TargetCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type TargetCallerSession struct {
	Contract *TargetCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts // Call options to use throughout this session
}

// TargetTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type TargetTransactorSession struct {
	Contract     *TargetTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// TargetRaw is an auto generated low-level Go binding around an Ethereum contract.
type TargetRaw struct {
	Contract *Target // Generic contract binding to access the raw methods on
}

// TargetCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type TargetCallerRaw struct {
	Contract *TargetCaller // Generic read-only contract binding to access the raw methods on
}

// TargetTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type TargetTransactorRaw struct {
	Contract *TargetTransactor // Generic write-only contract binding to access the raw methods on
}

// NewTarget creates a new instance of Target, bound to a specific deployed contract.
func NewTarget(address common.Address, backend bind.ContractBackend) (*Target, error) {
	contract, err := bindTarget(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &Target{TargetCaller: TargetCaller{contract: contract}, TargetTransactor: TargetTransactor{contract: contract}, TargetFilterer: TargetFilterer{contract: contract}}, nil
}

// NewTargetCaller creates a new read-only instance of Target, bound to a specific deployed contract.
func NewTargetCaller(address common.Address, caller bind.ContractCaller) (*TargetCaller, error) {
	contract, err := bindTarget(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &TargetCaller{contract: contract}, nil
}

// NewTargetTransactor creates a new write-only instance of Target, bound to a specific deployed contract.
func NewTargetTransactor(address common.Address, transactor bind.ContractTransactor) (*TargetTransactor, error) {
	contract, err := bindTarget(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &TargetTransactor{contract: contract}, nil
}

// NewTargetFilterer creates a new log filterer instance of Target, bound to a specific deployed contract.
func NewTargetFilterer(address common.Address, filterer bind.ContractFilterer) (*TargetFilterer, error) {
	contract, err := bindTarget(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &TargetFilterer{contract: contract}, nil
}

// bindTarget binds a generic wrapper to an already deployed contract.
func bindTarget(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := TargetMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Target *TargetRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _Target.Contract.TargetCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Target *TargetRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Target.Contract.TargetTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Target *TargetRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Target.Contract.TargetTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Target *TargetCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _Target.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Target *TargetTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Target.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Target *TargetTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Target.Contract.contract.Transact(opts, method, params...)
}

// Data is a free data retrieval call binding the contract method 0xf0ba8440.
//
// Solidity: function data(uint256 ) view returns(uint256)
func (_Target *TargetCaller) Data(opts *bind.CallOpts, arg0 *big.Int) (*big.Int, error) {
	var out []interface{}
	err := _Target.contract.Call(opts, &out, "data", arg0)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// Data is a free data retrieval call binding the contract method 0xf0ba8440.
//
// Solidity: function data(uint256 ) view returns(uint256)
func (_Target *TargetSession) Data(arg0 *big.Int) (*big.Int, error) {
	return _Target.Contract.Data(&_Target.CallOpts, arg0)
}

// Data is a free data retrieval call binding the contract method 0xf0ba8440.
//
// Solidity: function data(uint256 ) view returns(uint256)
func (_Target *TargetCallerSession) Data(arg0 *big.Int) (*big.Int, error) {
	return _Target.Contract.Data(&_Target.CallOpts, arg0)
}

// Process is a free data retrieval call binding the contract method 0xffb2c479.
//
// Solidity: function process(uint256 value) pure returns(uint256)
func (_Target *TargetCaller) Process(opts *bind.CallOpts, value *big.Int) (*big.Int, error) {
	var out []interface{}
	err := _Target.contract.Call(opts, &out, "process", value)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// Process is a free data retrieval call binding the contract method 0xffb2c479.
//
// Solidity: function process(uint256 value) pure returns(uint256)
func (_Target *TargetSession) Process(value *big.Int) (*big.Int, error) {
	return _Target.Contract.Process(&_Target.CallOpts, value)
}

// Process is a free data retrieval call binding the contract method 0xffb2c479.
//
// Solidity: function process(uint256 value) pure returns(uint256)
func (_Target *TargetCallerSession) Process(value *big.Int) (*big.Int, error) {
	return _Target.Contract.Process(&_Target.CallOpts, value)
}

// Store is a paid mutator transaction binding the contract method 0x6ed28ed0.
//
// Solidity: function store(uint256 key, uint256 value) returns()
func (_Target *TargetTransactor) Store(opts *bind.TransactOpts, key *big.Int, value *big.Int) (*types.Transaction, error) {
	return _Target.contract.Transact(opts, "store", key, value)
}

// Store is a paid mutator transaction binding the contract method 0x6ed28ed0.
//
// Solidity: function store(uint256 key, uint256 value) returns()
func (_Target *TargetSession) Store(key *big.Int, value *big.Int) (*types.Transaction, error) {
	return _Target.Contract.Store(&_Target.TransactOpts, key, value)
}

// Store is a paid mutator transaction binding the contract method 0x6ed28ed0.
//
// Solidity: function store(uint256 key, uint256 value) returns()
func (_Target *TargetTransactorSession) Store(key *big.Int, value *big.Int) (*types.Transaction, error) {
	return _Target.Contract.Store(&_Target.TransactOpts, key, value)
}
