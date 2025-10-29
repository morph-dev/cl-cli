package agent

import (
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/beacon/engine"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto/kzg4844"
	"github.com/ethereum/go-ethereum/log"
	"github.com/morph-dev/cl-cli/utils"
)

func (a *Agent) BuildBlock(autoConfirm bool) (err error) {
	var (
		latest           *common.Hash
		payloadId        *engine.PayloadID
		executionPayload *engine.ExecutionPayloadEnvelope
	)

	// Get current head of the chain
	if latest, err = a.ethClient.GetBlockByTag("latest"); err != nil {
		return err
	}

	// Start building new block
	payloadAttributes := createRandomPayloadAttributes()
	if payloadId, err = a.forkchoiceUpdated(*latest, &payloadAttributes); err != nil {
		return err
	}

	// Wait 1 second
	time.Sleep(time.Second)

	// Get new block
	if executionPayload, err = a.engineClient.GetPayload(*payloadId); err != nil {
		return err
	}
	utils.PrintJson("Chunks", executionPayload.ExecutionPayload.Chunks)

	// Ask whether to update head of the chain (or skip if autoConfirm)
	if !autoConfirm && !utils.PromptBool("Update head of the chain?") {
		return nil
	}

	log.Info("Updating head of the chain", "head", executionPayload.ExecutionPayload.BlockHash)

	// Send new payload
	if err = a.newPayload(executionPayload, payloadAttributes.BeaconRoot); err != nil {
		return err
	}

	// Update head of the chain
	if _, err = a.forkchoiceUpdated(executionPayload.ExecutionPayload.BlockHash, nil); err != nil {
		return err
	}

	return nil
}

func (a *Agent) forkchoiceUpdated(head common.Hash, payloadAttributes *engine.PayloadAttributes) (payloadId *engine.PayloadID, err error) {
	var forkChoiceResponse *engine.ForkChoiceResponse

	update := engine.ForkchoiceStateV1{HeadBlockHash: head}
	if forkChoiceResponse, err = a.engineClient.ForkchoiceUpdated(&update, payloadAttributes); err != nil {
		return nil, err
	}
	if forkChoiceResponse.PayloadStatus.Status != engine.VALID {
		return nil, fmt.Errorf("forkChoiceResponse status is not VALID, response: %v", forkChoiceResponse)
	}
	if payloadAttributes != nil && forkChoiceResponse.PayloadID == nil {
		return nil, fmt.Errorf("PayloadId is nil, response: %v", forkChoiceResponse)
	}

	return forkChoiceResponse.PayloadID, nil
}

func (a *Agent) newPayload(executionPayload *engine.ExecutionPayloadEnvelope, beaconRoot *common.Hash) error {
	hasher := sha256.New()

	blobHashes := make([]common.Hash, 0, len(executionPayload.BlobsBundle.Commitments))
	for _, commitment := range executionPayload.BlobsBundle.Commitments {
		blobHashes = append(blobHashes, kzg4844.CalcBlobHashV1(hasher, (*kzg4844.Commitment)(commitment)))
	}

	payloadStatus, err := a.engineClient.NewPayload(executionPayload.ExecutionPayload, blobHashes, beaconRoot, executionPayload.Requests)
	if err != nil {
		return err
	}
	if payloadStatus.Status != engine.VALID {
		return fmt.Errorf("NewPayload status is not VALID, response: %v", payloadStatus)
	}

	return nil
}

func createRandomPayloadAttributes() engine.PayloadAttributes {
	payloadAttributes := engine.PayloadAttributes{
		Timestamp:   uint64(time.Now().Unix()),
		Withdrawals: []*types.Withdrawal{},
		BeaconRoot:  &common.Hash{},
	}
	rand.Read(payloadAttributes.Random[:])
	rand.Read(payloadAttributes.SuggestedFeeRecipient[:])
	rand.Read(payloadAttributes.BeaconRoot[:])

	return payloadAttributes
}
