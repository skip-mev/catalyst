// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.13;

import {Script, console} from "forge-std/Script.sol";
import {Loader, Target} from "../src/Loader.sol";

contract LoaderScript is Script {
    Loader public loader;
    Target[] public targets;

    // Configuration
    uint256 constant NUM_TARGETS = 10; // Number of target contracts to deploy

    function setUp() public {}

    function run() public {
        vm.startBroadcast();

        // Deploy multiple Target contracts
        address[] memory targetAddresses = new address[](NUM_TARGETS);

        for (uint256 i = 0; i < NUM_TARGETS; i++) {
            Target target = new Target();
            targets.push(target);
            targetAddresses[i] = address(target);
            console.log("Deployed Target", i, "at:", address(target));
        }

        // Deploy Loader with all target addresses
        loader = new Loader(targetAddresses);
        console.log("Deployed Loader at:", address(loader));
        console.log("Total targets:", NUM_TARGETS);

        vm.stopBroadcast();
    }
}