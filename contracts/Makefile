deps:
	curl -L https://foundry.paradigm.xyz | bash
	foundryup

anvil-deploy-eigen-contracts:
	@echo "Deploying Eigen Contracts..."
	. scripts/anvil/deploy_eigen_contracts.sh

anvil-start:
	@echo "Starting Anvil..."
	. scripts/anvil/start.sh

anvil-deploy-aligned-layer:
	./tests/anvil/deploy-avs-save-anvil-state.sh

deploy-all-to-anvil-and-save-state: deploy-eigenlayer-contracts-to-anvil-and-save-state deploy-incredible-squaring-contracts-to-anvil-and-save-state