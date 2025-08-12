build:
	forge build --silent && jq '.abi' ./out/Loader.sol/Loader.json > loader.abi
	forge inspect src/Loader.sol:Loader bytecode > loader.bin
	abigen --abi loader.abi --bin loader.bin --pkg loader --out loader.go

build-target:
	forge build --silent && jq '.abi' ./out/Loader.sol/Target.json > target/target.abi
	forge inspect src/Loader.sol:Target bytecode > target/target.bin
	abigen --abi target/target.abi --bin target/target.bin --pkg target --out target/target.go