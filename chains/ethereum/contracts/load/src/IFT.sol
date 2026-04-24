// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.13;

// IFT is a minimal stub of the deployed IFT transfer contract.
// Catalyst only needs the ABI of iftTransfer to encode calldata; the contract
// itself is never deployed from this repo. Keep the signature in sync with the
// upstream deployed contract.
contract IFT {
    function iftTransfer(
        string calldata clientId,
        string calldata receiver,
        uint256 amount,
        uint64 timeoutTimestamp
    ) external {}
}
