build:
	forge build --silent && jq '.abi' ./out/Loader.sol/Loader.json > loader.abi
	forge inspect src/Loader.sol:Loader bytecode > loader.bin
	abigen --abi loader.abi --bin loader.bin --pkg loader --out loader.go

build-target:
	forge build --silent && jq '.abi' ./out/Loader.sol/Target.json > target/target.abi
	forge inspect src/Loader.sol:Target bytecode > target/target.bin
	abigen --abi target/target.abi --bin target/target.bin --pkg target --out target/target.go


build-weth:
	forge build --silent && jq '.abi' ./out/Weth.sol/Weth9.json > weth/weth.abi
	forge inspect src/Weth.sol:WETH9 bytecode > weth/weth.bin
	abigen --abi weth/weth.abi --bin weth/weth.bin --pkg weth --out weth/weth.go