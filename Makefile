deps:
	git submodule update --init --recursive
	make -C contracts deps
	go install github.com/maoueh/zap-pretty@latest

anvil-deploy-eigen-contracts:
	make -C contracts anvil-deploy-eigen-contracts

anvil-start:
	make -C contracts anvil-start

deploy-incredible-squaring-contracts-to-anvil-and-save-state: ## Deploy avs
	./tests/anvil/deploy-avs-save-anvil-state.sh

deploy-all-to-anvil-and-save-state: deploy-eigenlayer-contracts-to-anvil-and-save-state deploy-incredible-squaring-contracts-to-anvil-and-save-state ## deploy eigenlayer, shared avs contracts, and inc-sq contracts 