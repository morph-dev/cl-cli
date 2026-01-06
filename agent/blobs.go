package agent

import (
	"crypto/sha256"

	"github.com/ethereum/go-ethereum/beacon/engine"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto/kzg4844"
)

func getBlobHashes(blobsBundle *engine.BlobsBundle) []common.Hash {
	hasher := sha256.New()
	blobHashes := make([]common.Hash, 0, len(blobsBundle.Commitments))
	for _, commitment := range blobsBundle.Commitments {
		blobHash := kzg4844.CalcBlobHashV1(hasher, (*kzg4844.Commitment)(commitment))
		blobHashes = append(blobHashes, blobHash)
	}
	return blobHashes
}
