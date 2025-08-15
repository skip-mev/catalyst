// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.13;

interface ITarget {
    function process(uint256 value) external pure returns (uint256);
    function store(uint256 key, uint256 value) external;
}

contract Target {
    mapping(uint256 => uint256) public data;

    function process(uint256 value) external pure returns (uint256) {
        return value * 2;
    }

    function store(uint256 key, uint256 value) external {
        data[key] = value;
    }
}

contract Loader {
    mapping(uint256 => uint256) public storage1;
    ITarget[] public targets;

    constructor(address[] memory _targets) {
        for (uint256 i = 0; i < _targets.length; i++) {
            targets.push(ITarget(_targets[i]));
        }
    }

    // Test 1: Heavy storage writes
    function testStorageWrites(uint256 iterations) public {
        for (uint256 i = 0; i < iterations; i++) {
            storage1[i] = i * 2;
        }
    }

    // Test 2: Cross-contract calls (rotating through different contracts)
    function testCrossContractCalls(uint256 iterations) public {
        require(targets.length > 0, "No targets");

        for (uint256 i = 0; i < iterations; i++) {
            // Rotate through different target contracts to avoid caching
            ITarget target = targets[i % targets.length];

            uint256 result = target.process(i);
            target.store(i, result);
        }
    }

    // Test 3: Large calldata processing
    function testLargeCalldata(bytes calldata data) public returns (bytes32) {
        uint256 sum;
        for (uint256 i = 0; i < data.length; i++) {
            sum += uint8(data[i]);
        }
        storage1[0] = sum;
        return keccak256(data);
    }
}