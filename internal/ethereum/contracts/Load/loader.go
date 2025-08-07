// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package loader

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

// LoaderMetaData contains all meta data concerning the Loader contract.
var LoaderMetaData = &bind.MetaData{
	ABI: "[{\"type\":\"constructor\",\"inputs\":[{\"name\":\"_targets\",\"type\":\"address[]\",\"internalType\":\"address[]\"}],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"storage1\",\"inputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"targets\",\"inputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"contractITarget\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"testCrossContractCalls\",\"inputs\":[{\"name\":\"iterations\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"testLargeCalldata\",\"inputs\":[{\"name\":\"data\",\"type\":\"bytes\",\"internalType\":\"bytes\"}],\"outputs\":[{\"name\":\"\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"testStorageWrites\",\"inputs\":[{\"name\":\"iterations\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"}]",
	Bin: "0x608060405234801561000f575f5ffd5b50604051610b67380380610b6783398181016040528101906100319190610288565b5f5f90505b81518110156100c6576001828281518110610054576100536102cf565b5b6020026020010151908060018154018082558091505060019003905f5260205f20015f9091909190916101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff1602179055508080600101915050610036565b50506102fc565b5f604051905090565b5f5ffd5b5f5ffd5b5f5ffd5b5f601f19601f8301169050919050565b7f4e487b71000000000000000000000000000000000000000000000000000000005f52604160045260245ffd5b610128826100e2565b810181811067ffffffffffffffff82111715610147576101466100f2565b5b80604052505050565b5f6101596100cd565b9050610165828261011f565b919050565b5f67ffffffffffffffff821115610184576101836100f2565b5b602082029050602081019050919050565b5f5ffd5b5f73ffffffffffffffffffffffffffffffffffffffff82169050919050565b5f6101c282610199565b9050919050565b6101d2816101b8565b81146101dc575f5ffd5b50565b5f815190506101ed816101c9565b92915050565b5f6102056102008461016a565b610150565b9050808382526020820190506020840283018581111561022857610227610195565b5b835b81811015610251578061023d88826101df565b84526020840193505060208101905061022a565b5050509392505050565b5f82601f83011261026f5761026e6100de565b5b815161027f8482602086016101f3565b91505092915050565b5f6020828403121561029d5761029c6100d6565b5b5f82015167ffffffffffffffff8111156102ba576102b96100da565b5b6102c68482850161025b565b91505092915050565b7f4e487b71000000000000000000000000000000000000000000000000000000005f52603260045260245ffd5b61085e806103095f395ff3fe608060405234801561000f575f5ffd5b5060043610610055575f3560e01c80630a39ce02146100595780632f56c904146100895780634035df51146100b95780637d3b4e4d146100d5578063d620bcf1146100f1575b5f5ffd5b610073600480360381019061006e9190610408565b610121565b60405161008091906104ad565b60405180910390f35b6100a3600480360381019061009e9190610527565b61015c565b6040516100b0919061058a565b60405180910390f35b6100d360048036038101906100ce9190610408565b6101e2565b005b6100ef60048036038101906100ea9190610408565b61037a565b005b61010b60048036038101906101069190610408565b6103b9565b60405161011891906105b2565b60405180910390f35b60018181548110610130575f80fd5b905f5260205f20015f915054906101000a900473ffffffffffffffffffffffffffffffffffffffff1681565b5f5f5f5f90505b848490508110156101ab57848482818110610181576101806105cb565b5b9050013560f81c60f81b60f81c60ff168261019c9190610625565b91508080600101915050610163565b50805f5f5f81526020019081526020015f208190555083836040516101d1929190610694565b604051809103902091505092915050565b5f60018054905011610229576040517f08c379a000000000000000000000000000000000000000000000000000000000815260040161022090610706565b60405180910390fd5b5f5f90505b81811015610376575f60018080549050836102499190610751565b8154811061025a576102596105cb565b5b905f5260205f20015f9054906101000a900473ffffffffffffffffffffffffffffffffffffffff1690505f8173ffffffffffffffffffffffffffffffffffffffff1663ffb2c479846040518263ffffffff1660e01b81526004016102be91906105b2565b602060405180830381865afa1580156102d9573d5f5f3e3d5ffd5b505050506040513d601f19601f820116820180604052508101906102fd9190610795565b90508173ffffffffffffffffffffffffffffffffffffffff16636ed28ed084836040518363ffffffff1660e01b815260040161033a9291906107c0565b5f604051808303815f87803b158015610351575f5ffd5b505af1158015610363573d5f5f3e3d5ffd5b505050505050808060010191505061022e565b5050565b5f5f90505b818110156103b55760028161039491906107e7565b5f5f8381526020019081526020015f2081905550808060010191505061037f565b5050565b5f602052805f5260405f205f915090505481565b5f5ffd5b5f5ffd5b5f819050919050565b6103e7816103d5565b81146103f1575f5ffd5b50565b5f81359050610402816103de565b92915050565b5f6020828403121561041d5761041c6103cd565b5b5f61042a848285016103f4565b91505092915050565b5f73ffffffffffffffffffffffffffffffffffffffff82169050919050565b5f819050919050565b5f61047561047061046b84610433565b610452565b610433565b9050919050565b5f6104868261045b565b9050919050565b5f6104978261047c565b9050919050565b6104a78161048d565b82525050565b5f6020820190506104c05f83018461049e565b92915050565b5f5ffd5b5f5ffd5b5f5ffd5b5f5f83601f8401126104e7576104e66104c6565b5b8235905067ffffffffffffffff811115610504576105036104ca565b5b6020830191508360018202830111156105205761051f6104ce565b5b9250929050565b5f5f6020838503121561053d5761053c6103cd565b5b5f83013567ffffffffffffffff81111561055a576105596103d1565b5b610566858286016104d2565b92509250509250929050565b5f819050919050565b61058481610572565b82525050565b5f60208201905061059d5f83018461057b565b92915050565b6105ac816103d5565b82525050565b5f6020820190506105c55f8301846105a3565b92915050565b7f4e487b71000000000000000000000000000000000000000000000000000000005f52603260045260245ffd5b7f4e487b71000000000000000000000000000000000000000000000000000000005f52601160045260245ffd5b5f61062f826103d5565b915061063a836103d5565b9250828201905080821115610652576106516105f8565b5b92915050565b5f81905092915050565b828183375f83830152505050565b5f61067b8385610658565b9350610688838584610662565b82840190509392505050565b5f6106a0828486610670565b91508190509392505050565b5f82825260208201905092915050565b7f4e6f2074617267657473000000000000000000000000000000000000000000005f82015250565b5f6106f0600a836106ac565b91506106fb826106bc565b602082019050919050565b5f6020820190508181035f83015261071d816106e4565b9050919050565b7f4e487b71000000000000000000000000000000000000000000000000000000005f52601260045260245ffd5b5f61075b826103d5565b9150610766836103d5565b92508261077657610775610724565b5b828206905092915050565b5f8151905061078f816103de565b92915050565b5f602082840312156107aa576107a96103cd565b5b5f6107b784828501610781565b91505092915050565b5f6040820190506107d35f8301856105a3565b6107e060208301846105a3565b9392505050565b5f6107f1826103d5565b91506107fc836103d5565b925082820261080a816103d5565b91508282048414831517610821576108206105f8565b5b509291505056fea26469706673582212203e3c1531cc8f0cf6267431016a7a0cfc7c9916557eb5ee060d9bc983683a7cfb64736f6c634300081e0033",
}

// LoaderABI is the input ABI used to generate the binding from.
// Deprecated: Use LoaderMetaData.ABI instead.
var LoaderABI = LoaderMetaData.ABI

// LoaderBin is the compiled bytecode used for deploying new contracts.
// Deprecated: Use LoaderMetaData.Bin instead.
var LoaderBin = LoaderMetaData.Bin

// DeployLoader deploys a new Ethereum contract, binding an instance of Loader to it.
func DeployLoader(auth *bind.TransactOpts, backend bind.ContractBackend, _targets []common.Address) (common.Address, *types.Transaction, *Loader, error) {
	parsed, err := LoaderMetaData.GetAbi()
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	if parsed == nil {
		return common.Address{}, nil, nil, errors.New("GetABI returned nil")
	}

	address, tx, contract, err := bind.DeployContract(auth, *parsed, common.FromHex(LoaderBin), backend, _targets)
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	return address, tx, &Loader{LoaderCaller: LoaderCaller{contract: contract}, LoaderTransactor: LoaderTransactor{contract: contract}, LoaderFilterer: LoaderFilterer{contract: contract}}, nil
}

// Loader is an auto generated Go binding around an Ethereum contract.
type Loader struct {
	LoaderCaller     // Read-only binding to the contract
	LoaderTransactor // Write-only binding to the contract
	LoaderFilterer   // Log filterer for contract events
}

// LoaderCaller is an auto generated read-only Go binding around an Ethereum contract.
type LoaderCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// LoaderTransactor is an auto generated write-only Go binding around an Ethereum contract.
type LoaderTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// LoaderFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type LoaderFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// LoaderSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type LoaderSession struct {
	Contract     *Loader           // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// LoaderCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type LoaderCallerSession struct {
	Contract *LoaderCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts // Call options to use throughout this session
}

// LoaderTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type LoaderTransactorSession struct {
	Contract     *LoaderTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// LoaderRaw is an auto generated low-level Go binding around an Ethereum contract.
type LoaderRaw struct {
	Contract *Loader // Generic contract binding to access the raw methods on
}

// LoaderCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type LoaderCallerRaw struct {
	Contract *LoaderCaller // Generic read-only contract binding to access the raw methods on
}

// LoaderTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type LoaderTransactorRaw struct {
	Contract *LoaderTransactor // Generic write-only contract binding to access the raw methods on
}

// NewLoader creates a new instance of Loader, bound to a specific deployed contract.
func NewLoader(address common.Address, backend bind.ContractBackend) (*Loader, error) {
	contract, err := bindLoader(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &Loader{LoaderCaller: LoaderCaller{contract: contract}, LoaderTransactor: LoaderTransactor{contract: contract}, LoaderFilterer: LoaderFilterer{contract: contract}}, nil
}

// NewLoaderCaller creates a new read-only instance of Loader, bound to a specific deployed contract.
func NewLoaderCaller(address common.Address, caller bind.ContractCaller) (*LoaderCaller, error) {
	contract, err := bindLoader(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &LoaderCaller{contract: contract}, nil
}

// NewLoaderTransactor creates a new write-only instance of Loader, bound to a specific deployed contract.
func NewLoaderTransactor(address common.Address, transactor bind.ContractTransactor) (*LoaderTransactor, error) {
	contract, err := bindLoader(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &LoaderTransactor{contract: contract}, nil
}

// NewLoaderFilterer creates a new log filterer instance of Loader, bound to a specific deployed contract.
func NewLoaderFilterer(address common.Address, filterer bind.ContractFilterer) (*LoaderFilterer, error) {
	contract, err := bindLoader(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &LoaderFilterer{contract: contract}, nil
}

// bindLoader binds a generic wrapper to an already deployed contract.
func bindLoader(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := LoaderMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Loader *LoaderRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _Loader.Contract.LoaderCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Loader *LoaderRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Loader.Contract.LoaderTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Loader *LoaderRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Loader.Contract.LoaderTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Loader *LoaderCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _Loader.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Loader *LoaderTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Loader.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Loader *LoaderTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Loader.Contract.contract.Transact(opts, method, params...)
}

// Storage1 is a free data retrieval call binding the contract method 0xd620bcf1.
//
// Solidity: function storage1(uint256 ) view returns(uint256)
func (_Loader *LoaderCaller) Storage1(opts *bind.CallOpts, arg0 *big.Int) (*big.Int, error) {
	var out []interface{}
	err := _Loader.contract.Call(opts, &out, "storage1", arg0)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// Storage1 is a free data retrieval call binding the contract method 0xd620bcf1.
//
// Solidity: function storage1(uint256 ) view returns(uint256)
func (_Loader *LoaderSession) Storage1(arg0 *big.Int) (*big.Int, error) {
	return _Loader.Contract.Storage1(&_Loader.CallOpts, arg0)
}

// Storage1 is a free data retrieval call binding the contract method 0xd620bcf1.
//
// Solidity: function storage1(uint256 ) view returns(uint256)
func (_Loader *LoaderCallerSession) Storage1(arg0 *big.Int) (*big.Int, error) {
	return _Loader.Contract.Storage1(&_Loader.CallOpts, arg0)
}

// Targets is a free data retrieval call binding the contract method 0x0a39ce02.
//
// Solidity: function targets(uint256 ) view returns(address)
func (_Loader *LoaderCaller) Targets(opts *bind.CallOpts, arg0 *big.Int) (common.Address, error) {
	var out []interface{}
	err := _Loader.contract.Call(opts, &out, "targets", arg0)

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Targets is a free data retrieval call binding the contract method 0x0a39ce02.
//
// Solidity: function targets(uint256 ) view returns(address)
func (_Loader *LoaderSession) Targets(arg0 *big.Int) (common.Address, error) {
	return _Loader.Contract.Targets(&_Loader.CallOpts, arg0)
}

// Targets is a free data retrieval call binding the contract method 0x0a39ce02.
//
// Solidity: function targets(uint256 ) view returns(address)
func (_Loader *LoaderCallerSession) Targets(arg0 *big.Int) (common.Address, error) {
	return _Loader.Contract.Targets(&_Loader.CallOpts, arg0)
}

// TestCrossContractCalls is a paid mutator transaction binding the contract method 0x4035df51.
//
// Solidity: function testCrossContractCalls(uint256 iterations) returns()
func (_Loader *LoaderTransactor) TestCrossContractCalls(opts *bind.TransactOpts, iterations *big.Int) (*types.Transaction, error) {
	return _Loader.contract.Transact(opts, "testCrossContractCalls", iterations)
}

// TestCrossContractCalls is a paid mutator transaction binding the contract method 0x4035df51.
//
// Solidity: function testCrossContractCalls(uint256 iterations) returns()
func (_Loader *LoaderSession) TestCrossContractCalls(iterations *big.Int) (*types.Transaction, error) {
	return _Loader.Contract.TestCrossContractCalls(&_Loader.TransactOpts, iterations)
}

// TestCrossContractCalls is a paid mutator transaction binding the contract method 0x4035df51.
//
// Solidity: function testCrossContractCalls(uint256 iterations) returns()
func (_Loader *LoaderTransactorSession) TestCrossContractCalls(iterations *big.Int) (*types.Transaction, error) {
	return _Loader.Contract.TestCrossContractCalls(&_Loader.TransactOpts, iterations)
}

// TestLargeCalldata is a paid mutator transaction binding the contract method 0x2f56c904.
//
// Solidity: function testLargeCalldata(bytes data) returns(bytes32)
func (_Loader *LoaderTransactor) TestLargeCalldata(opts *bind.TransactOpts, data []byte) (*types.Transaction, error) {
	return _Loader.contract.Transact(opts, "testLargeCalldata", data)
}

// TestLargeCalldata is a paid mutator transaction binding the contract method 0x2f56c904.
//
// Solidity: function testLargeCalldata(bytes data) returns(bytes32)
func (_Loader *LoaderSession) TestLargeCalldata(data []byte) (*types.Transaction, error) {
	return _Loader.Contract.TestLargeCalldata(&_Loader.TransactOpts, data)
}

// TestLargeCalldata is a paid mutator transaction binding the contract method 0x2f56c904.
//
// Solidity: function testLargeCalldata(bytes data) returns(bytes32)
func (_Loader *LoaderTransactorSession) TestLargeCalldata(data []byte) (*types.Transaction, error) {
	return _Loader.Contract.TestLargeCalldata(&_Loader.TransactOpts, data)
}

// TestStorageWrites is a paid mutator transaction binding the contract method 0x7d3b4e4d.
//
// Solidity: function testStorageWrites(uint256 iterations) returns()
func (_Loader *LoaderTransactor) TestStorageWrites(opts *bind.TransactOpts, iterations *big.Int) (*types.Transaction, error) {
	return _Loader.contract.Transact(opts, "testStorageWrites", iterations)
}

// TestStorageWrites is a paid mutator transaction binding the contract method 0x7d3b4e4d.
//
// Solidity: function testStorageWrites(uint256 iterations) returns()
func (_Loader *LoaderSession) TestStorageWrites(iterations *big.Int) (*types.Transaction, error) {
	return _Loader.Contract.TestStorageWrites(&_Loader.TransactOpts, iterations)
}

// TestStorageWrites is a paid mutator transaction binding the contract method 0x7d3b4e4d.
//
// Solidity: function testStorageWrites(uint256 iterations) returns()
func (_Loader *LoaderTransactorSession) TestStorageWrites(iterations *big.Int) (*types.Transaction, error) {
	return _Loader.Contract.TestStorageWrites(&_Loader.TransactOpts, iterations)
}
