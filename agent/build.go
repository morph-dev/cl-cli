package agent

import (
	"crypto/rand"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/beacon/engine"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/morph-dev/cl-cli/utils"
)

// Create new block using chunks and update the head
func (a *Agent) BuildBlock(autoConfirm bool, blockDuration time.Duration) (err error) {
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

	// Wait for the block
	time.Sleep(blockDuration)

	// Get new block
	if executionPayload, err = a.engineClient.GetPayload(*payloadId); err != nil {
		return err
	}
	for _, chunk := range executionPayload.Chunks {
		header := &chunk.ChunkHeader
		utils.PrintJson(fmt.Sprintf("Chunk %d txCound: %d", header.Index, len(chunk.Transactions)), header)
	}

	// Ask whether to update head of the chain (or skip if autoConfirm)
	if !autoConfirm && !utils.PromptBool("Update head of the chain?") {
		return nil
	}

	log.Info("Updating head of the chain", "head", executionPayload.ExecutionPayload.BlockHash)

	// Send new payload
	if err = a.newHead(executionPayload, *payloadAttributes.BeaconRoot); err != nil {
		return err
	}

	return nil
}

// Updates the head of the chain and/or starts building new block
func (a *Agent) forkchoiceUpdated(head common.Hash, payloadAttributes *engine.PayloadAttributes) (payloadId *engine.PayloadID, err error) {
	var forkChoiceResponse *engine.ForkChoiceResponse

	update := engine.ForkchoiceStateV1{HeadBlockHash: head}
	if forkChoiceResponse, err = a.engineClient.ForkchoiceUpdated(&update, payloadAttributes); err != nil {
		return nil, err
	}
	if forkChoiceResponse.PayloadStatus.Status != engine.VALID {
		return nil, fmt.Errorf(
			"forkChoiceResponse status is not VALID, status: %v error: %+v",
			forkChoiceResponse.PayloadStatus.Status,
			forkChoiceResponse.PayloadStatus.ValidationError,
		)
	}
	if payloadAttributes != nil && forkChoiceResponse.PayloadID == nil {
		return nil, fmt.Errorf("PayloadId is nil, response: %v", forkChoiceResponse)
	}

	return forkChoiceResponse.PayloadID, nil
}

// Sends new block to the EL and set it as head.
func (a *Agent) newHead(payload *engine.ExecutionPayloadEnvelope, beaconRoot common.Hash) error {
	blockHash := payload.ExecutionPayload.BlockHash

	// validate new block using chunks
	if err := a.newBlock(payload, beaconRoot); err != nil {
		return err
	}

	// Update head of the chain
	if _, err := a.forkchoiceUpdated(blockHash, nil); err != nil {
		return err
	}

	return nil
}

// Sends new block in pieces (header, cals, chunks) to the EL and validates it
func (a *Agent) newBlock(payload *engine.ExecutionPayloadEnvelope, beaconRoot common.Hash) error {
	blockHash := payload.ExecutionPayload.BlockHash
	log.Info("NewBlock", "blockHash", blockHash)

	if err := a.newBlockHeader(payload, beaconRoot); err != nil {
		return err
	}

	for _, chunkRequest := range randomizeChunkRequests(len(payload.Chunks)) {
		var err error
		switch chunkRequest.requestType {
		case requestTypeCal:
			err = a.sendCal(blockHash, payload.Chunks[chunkRequest.chunkIndex])
		case requestTypeChunk:
			err = a.executeChunk(blockHash, payload.Chunks[chunkRequest.chunkIndex])
		}
		if err != nil {
			return err
		}
	}

	if err := a.finalize(blockHash); err != nil {
		return err
	}

	return nil
}

// Tells EL that there is new BlockHeader
func (a *Agent) newBlockHeader(payload *engine.ExecutionPayloadEnvelope, beaconRoot common.Hash) error {
	payloadStatus, err := a.engineClient.NewBlockHeader(
		*payload.ExecutionPayload,
		beaconRoot,
		getBlobHashes(payload.BlobsBundle),
		payload.Requests,
		len(payload.Chunks),
	)
	if err != nil {
		return err
	}
	if payloadStatus.Status != engine.ACCEPTED {
		return fmt.Errorf(
			"NewBlockHeader status is not ACCEPTED, status: %v error: %+v",
			payloadStatus.Status,
			payloadStatus.ValidationError,
		)
	}
	return nil
}

// Sends CAL to EL
func (a *Agent) sendCal(blockHash common.Hash, chunk engine.ExecutionChunk) error {
	payloadStatus, err := a.engineClient.NewChunkAccessList(
		blockHash,
		chunk.ChunkHeader.Index,
		*chunk.ChunkAccessList,
	)
	if err != nil {
		return err
	}
	if payloadStatus.Status != engine.ACCEPTED {
		return fmt.Errorf(
			"NewChunkAccessList status is not ACCEPTED, status: %v error: %+v",
			payloadStatus.Status,
			payloadStatus.ValidationError,
		)
	}
	return nil
}

// Sends chunk to EL, executes and validates it
func (a *Agent) executeChunk(blockHash common.Hash, chunk engine.ExecutionChunk) error {
	chunkBody := engine.ExecutionChunkBody{
		ChunkHeader:  chunk.ChunkHeader,
		Transactions: chunk.Transactions,
		Withdrawals:  chunk.Withdrawals,
	}
	payloadStatus, err := a.engineClient.ExecuteChunk(blockHash, chunkBody)
	if err != nil {
		return err
	}
	if payloadStatus.Status != engine.VALID {
		return fmt.Errorf(
			"ExecuteChunk status is not VALID, status: %v error: %+v",
			payloadStatus.Status,
			payloadStatus.ValidationError,
		)
	}
	return nil
}

// Finalizes the EL block
func (a *Agent) finalize(blockHash common.Hash) error {
	payloadStatus, err := a.engineClient.FinalizeBlock(blockHash)
	if err != nil {
		return err
	}
	if payloadStatus.Status != engine.VALID {
		return fmt.Errorf(
			"FinalizeBlock status is not VALID, status: %v error: %+v",
			payloadStatus.Status,
			*payloadStatus.ValidationError,
		)
	}
	return nil
}

// Creates PayloadAttributes used for creating new block
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
