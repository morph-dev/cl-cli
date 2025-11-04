package agent

import (
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/beacon/engine"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/types/bal"
	"github.com/ethereum/go-ethereum/crypto/kzg4844"
	"github.com/ethereum/go-ethereum/log"
	"github.com/morph-dev/cl-cli/utils"
)

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
	utils.PrintJson("Chunks", executionPayload.ExecutionPayload.Chunks)

	// Ask whether to update head of the chain (or skip if autoConfirm)
	if !autoConfirm && !utils.PromptBool("Update head of the chain?") {
		return nil
	}

	log.Info("Updating head of the chain", "head", executionPayload.ExecutionPayload.BlockHash)

	// Send new payload
	if err = a.newHeadFromExecutionPayload(executionPayload, payloadAttributes.BeaconRoot); err != nil {
		return err
	}

	return nil
}

func (a *Agent) BuildBlockWithChunks(autoConfirm bool, chunkPayloads uint, chunkDuration time.Duration) (err error) {
	var (
		latest         *common.Hash
		payloadId      *engine.PayloadID
		chunksEnvelope *engine.ChunksEnvelope
		chunks         []*engine.ChunkPayload
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

	for chunkIndex := uint(0); chunkIndex < chunkPayloads; chunkIndex++ {
		time.Sleep(chunkDuration)

		finalize := chunkIndex == chunkPayloads-1
		if chunksEnvelope, err = a.engineClient.GetChunk(*payloadId, finalize); err != nil {
			return err
		}
		chunks = append(chunks, chunksEnvelope.Chunks...)
		payloadId = chunksEnvelope.PayloadID
	}

	header := chunksEnvelope.Header
	if header == nil {
		return fmt.Errorf("Header of the last chunksEnvelope is nil")
	}
	utils.PrintJson("Block header", header)

	// Ask whether to update head of the chain (or skip if autoConfirm)
	if !autoConfirm && !utils.PromptBool("Update head of the chain?") {
		return nil
	}

	if err = a.newHeadFromChunks(chunks, header); err != nil {
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

// Send new block and set it as head.
func (a *Agent) newHead(
	executableData *engine.ExecutableData,
	blobCommitments []hexutil.Bytes,
	beaconRoot *common.Hash,
	requests [][]byte,
) error {
	hasher := sha256.New()

	blobHashes := make([]common.Hash, 0, len(blobCommitments))
	for _, commitment := range blobCommitments {
		blobHashes = append(blobHashes, kzg4844.CalcBlobHashV1(hasher, (*kzg4844.Commitment)(commitment)))
	}

	payloadStatus, err := a.engineClient.NewPayload(executableData, blobHashes, beaconRoot, requests)
	if err != nil {
		return err
	}
	if payloadStatus.Status != engine.VALID {
		return fmt.Errorf("NewPayload status is not VALID, response: %v", payloadStatus)
	}

	if _, err = a.forkchoiceUpdated(executableData.BlockHash, nil); err != nil {
		return err
	}

	return nil
}

func (a *Agent) newHeadFromExecutionPayload(executionPayload *engine.ExecutionPayloadEnvelope, beaconRoot *common.Hash) error {
	return a.newHead(
		executionPayload.ExecutionPayload,
		executionPayload.BlobsBundle.Commitments,
		beaconRoot,
		executionPayload.Requests,
	)
}

func (a *Agent) newHeadFromChunks(chunks []*engine.ChunkPayload, header *types.Header) error {
	executableData, blobsBundle, requests := aggregateChunks(chunks, header)
	return a.newHead(executableData, blobsBundle.Commitments, header.ParentBeaconRoot, requests)
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

func aggregateChunks(chunks []*engine.ChunkPayload, header *types.Header) (*engine.ExecutableData, *engine.BlobsBundle, [][]byte) {
	var (
		chunkHeaders []*types.ChunkHeader = make([]*types.ChunkHeader, 0, len(chunks))
		transactions [][]byte             = make([][]byte, 0)
		withdrawals  types.Withdrawals    = make(types.Withdrawals, 0)
		bal          *bal.BlockAccessList
		blobsBundle  *engine.BlobsBundle
		requests     [][]byte
	)

	blobsBundle = &engine.BlobsBundle{
		Commitments: make([]hexutil.Bytes, 0),
		Blobs:       make([]hexutil.Bytes, 0),
		Proofs:      make([]hexutil.Bytes, 0),
	}
	for _, chunk := range chunks {
		log.Info("Aggregating chunk", "blobs.commitments", len(chunk.BlobsBundle.Commitments))
		chunkHeaders = append(chunkHeaders, chunk.Header)
		transactions = append(transactions, chunk.Transactions...)
		withdrawals = append(withdrawals, chunk.Withdrawals...)

		blobsBundle.Blobs = append(blobsBundle.Blobs, chunk.BlobsBundle.Blobs...)
		blobsBundle.Commitments = append(blobsBundle.Commitments, chunk.BlobsBundle.Commitments...)
		blobsBundle.Proofs = append(blobsBundle.Proofs, chunk.BlobsBundle.Proofs...)
	}

	lastChunk := chunks[len(chunks)-1]
	bal = lastChunk.AccessList
	if lastChunk.Requests != nil {
		requests = lastChunk.Requests
	} else {
		requests = make([][]byte, 0)
	}

	executableData := &engine.ExecutableData{
		ParentHash:       header.ParentHash,
		FeeRecipient:     header.Coinbase,
		StateRoot:        header.Root,
		ReceiptsRoot:     header.ReceiptHash,
		LogsBloom:        header.Bloom.Bytes(),
		Random:           header.MixDigest,
		Number:           header.Number.Uint64(),
		GasLimit:         header.GasLimit,
		GasUsed:          header.GasUsed,
		Timestamp:        header.Time,
		ExtraData:        header.Extra,
		BaseFeePerGas:    header.BaseFee,
		BlockHash:        header.Hash(),
		Transactions:     transactions,
		Withdrawals:      withdrawals,
		BlobGasUsed:      header.BlobGasUsed,
		ExcessBlobGas:    header.ExcessBlobGas,
		BlockAccessList:  bal,
		ExecutionWitness: nil, // TODO
		Chunks:           chunkHeaders,
	}
	return executableData, blobsBundle, requests
}
