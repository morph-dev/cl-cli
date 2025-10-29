package client

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rpc"
)

type EthClient struct {
	*rpc.Client
}

func NewEthClient(url string) (*EthClient, error) {
	client, err := rpc.Dial(url)
	if err != nil {
		return nil, err
	}

	var clientVersion string
	if err := client.Call(&clientVersion, "web3_clientVersion"); err != nil {
		return nil, err
	}

	log.Debug("Eth client", "version", clientVersion)

	return &EthClient{client}, nil
}

func (c *EthClient) GetBlockByTag(tag string) (*common.Hash, error) {
	var block struct {
		Hash common.Hash `json:"hash"`
	}
	if err := c.Call(&block, "eth_getBlockByNumber", tag, false /* =fullTx */); err != nil {
		return nil, err
	}
	return &block.Hash, nil
}
