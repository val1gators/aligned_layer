package sp1_test

import (
	"os"
	"testing"

	sp1 "github.com/yetanotherco/aligned_layer/operator/sp1_groth16"
)

func TestFibonacciSp1Groth16ProofVerifies(t *testing.T) {
	proofFile, err := os.Open("../../task_sender/test_examples/sp1_groth16/proof-with-pis.json")
	if err != nil {
		t.Errorf("could not open proof file: %s", err)
	}
	proofBytes := make([]byte, sp1.MaxProofSize)
	nReadProofBytes, err := proofFile.Read(proofBytes)
	if err != nil {
		t.Errorf("could not read bytes from file")
	}

	elfFile, err := os.Open("../../task_sender/test_examples/sp1_groth16/elf/riscv32im-succinct-zkvm-elf")
	if err != nil {
		t.Errorf("could not open proof file: %s", err)
	}
	elfBytes := make([]byte, sp1.MaxElfBufferSize)
	nReadElfBytes, err := elfFile.Read(elfBytes)
	if err != nil {
		t.Errorf("could not read bytes from file")
	}

	if !sp1.VerifySp1Proof(([sp1.MaxProofSize]byte)(proofBytes), uint(nReadProofBytes), ([sp1.MaxElfBufferSize]byte)(elfBytes), uint(nReadElfBytes)) {
		t.Errorf("proof did not verify")
	}
}
