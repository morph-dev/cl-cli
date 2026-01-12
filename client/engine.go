package client

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/beacon/engine"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types/bal"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/rpc"
)

type EngineClient struct {
	*rpc.Client
}

func NewEngineClient(url, jwtFilename string) (*EngineClient, error) {
	var jwtSecret [32]byte
	if jwt, err := node.ObtainJWTSecret(jwtFilename); err == nil {
		copy(jwtSecret[:], jwt)
	} else {
		return nil, err
	}

	client, err := rpc.DialOptions(context.Background(), url, rpc.WithHTTPAuth(node.NewJWTAuth(jwtSecret)))
	if err != nil {
		return nil, err
	}

	var clientVersions []engine.ClientVersionV1
	if err := client.Call(&clientVersions, "engine_getClientVersionV1", engine.ClientVersionV1{}); err != nil {
		return nil, err
	}
	log.Debug("Engine API client", "version", clientVersions)

	return &EngineClient{client}, nil
}

func (c *EngineClient) ForkchoiceUpdated(
	update *engine.ForkchoiceStateV1,
	payloadAttributes *engine.PayloadAttributes,
) (*engine.ForkChoiceResponse, error) {
	var forkchoiceResponse engine.ForkChoiceResponse
	log.Debug("engine_forkchoiceUpdatedV3", "head", update.HeadBlockHash)
	if err := c.Call(&forkchoiceResponse, "engine_forkchoiceUpdatedV3", update, payloadAttributes); err != nil {
		return nil, err
	}

	log.Info(
		"engine_forkchoiceUpdatedV3",
		"head", update.HeadBlockHash,
		"status", forkchoiceResponse.PayloadStatus.Status,
		"latestValidHash", forkchoiceResponse.PayloadStatus.LatestValidHash,
		"payloadId", forkchoiceResponse.PayloadID,
	)
	return &forkchoiceResponse, nil
}

func (c *EngineClient) GetPayload(payloadId engine.PayloadID) (*engine.ExecutionPayloadEnvelope, error) {
	var method string
	switch {
	case payloadId.Is(engine.PayloadV4):
		method = "engine_getPayloadV6"
	case payloadId.Is(engine.PayloadV3):
		method = "engine_getPayloadV5"
	case payloadId.Is(engine.PayloadV2):
		method = "engine_getPayloadV2"
	case payloadId.Is(engine.PayloadV1):
		method = "engine_getPayloadV1"
	default:
		return nil, fmt.Errorf("Unknown payload version: %v", payloadId)
	}

	log.Debug(method, "payloadId", payloadId)
	var executionPayload engine.ExecutionPayloadEnvelope
	if err := c.Call(&executionPayload, method, payloadId); err != nil {
		return nil, err
	}
	log.Info(
		method,
		"payloadId", payloadId,
		"number", executionPayload.ExecutionPayload.Number,
		"hash", executionPayload.ExecutionPayload.BlockHash,
		"parent", executionPayload.ExecutionPayload.ParentHash,
		"blobCount", len(executionPayload.BlobsBundle.Blobs),
		"chunkCount", len(executionPayload.Chunks),
	)
	return &executionPayload, nil
}

func (c *EngineClient) NewPayload(
	params *engine.ExecutableData,
	blobHashes []common.Hash,
	beaconRoot *common.Hash,
	executionRequests [][]byte,
) (*engine.PayloadStatusV1, error) {
	log.Debug(
		"engine_newPayloadV5",
		"blockHash", params.BlockHash,
	)
	var payloadStatus engine.PayloadStatusV1
	if err := c.Call(&payloadStatus, "engine_newPayloadV5", params, blobHashes, beaconRoot, executionRequests); err != nil {
		return nil, err
	}
	log.Info(
		"engine_newPayloadV5",
		"blockHash", params.BlockHash,
		"status", payloadStatus.Status,
		"latestValidHash", payloadStatus.LatestValidHash,
	)
	return &payloadStatus, nil
}

func (c *EngineClient) NewBlockHeader(
	data engine.ExecutableData,
	beaconRoot common.Hash,
	blobHashes []common.Hash,
	requests [][]byte,
	chunkCount int,
) (*engine.PayloadStatusV1, error) {
	log.Debug(
		"engine_newBlockHeaderV1",
		"blockHash", data.BlockHash,
		"chunkCount", chunkCount,
	)
	var payloadStatus engine.PayloadStatusV1
	if err := c.Call(&payloadStatus, "engine_newBlockHeaderV1", data, beaconRoot, blobHashes, requests, chunkCount); err != nil {
		return nil, err
	}
	log.Info(
		"engine_newBlockHeaderV1",
		"blockHash", data.BlockHash,
		"chunkCount", chunkCount,
		"status", payloadStatus.Status,
	)
	return &payloadStatus, nil
}

func (c *EngineClient) NewChunkAccessList(
	blockHash common.Hash,
	chunkIndex uint16,
	cal bal.BlockAccessList,
) (*engine.PayloadStatusV1, error) {
	log.Debug(
		"engine_newChunkAccessListV1",
		"blockHash", blockHash,
		"chunk", chunkIndex,
	)
	var payloadStatus engine.PayloadStatusV1
	if err := c.Call(&payloadStatus, "engine_newChunkAccessListV1", blockHash, chunkIndex, cal); err != nil {
		return nil, err
	}
	log.Info(
		"engine_newChunkAccessListV1",
		"blockHash", blockHash,
		"chunk", chunkIndex,
		"status", payloadStatus.Status,
	)
	return &payloadStatus, nil
}

func (c *EngineClient) ExecuteChunk(
	blockHash common.Hash,
	chunk engine.ExecutionChunk,
) (*engine.PayloadStatusV1, error) {
	log.Debug(
		"engine_executeChunkV1",
		"blockHash", blockHash,
		"chunk", chunk.ChunkHeader.Index,
		"txCound", len(chunk.Transactions),
	)
	var payloadStatus engine.PayloadStatusV1
	if err := c.Call(&payloadStatus, "engine_executeChunkV1", blockHash, chunk); err != nil {
		return nil, err
	}
	log.Info(
		"engine_executeChunkV1",
		"blockHash", blockHash,
		"chunk", chunk.ChunkHeader.Index,
		"txCound", len(chunk.Transactions),
		"status", payloadStatus.Status,
	)
	return &payloadStatus, nil
}

func (c *EngineClient) FinalizeBlock(
	blockHash common.Hash,
) (*engine.PayloadStatusV1, error) {
	log.Debug(
		"engine_finalizeBlockV1",
		"blockHash", blockHash,
	)
	var payloadStatus engine.PayloadStatusV1
	if err := c.Call(&payloadStatus, "engine_finalizeBlockV1", blockHash); err != nil {
		return nil, err
	}
	log.Info(
		"engine_finalizeBlockV1",
		"blockHash", blockHash,
		"status", payloadStatus.Status,
	)
	return &payloadStatus, nil
}
