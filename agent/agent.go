package agent

import (
	"github.com/morph-dev/cl-cli/client"
)

type Agent struct {
	ethClient    *client.EthClient
	engineClient *client.EngineClient
}

func NewAgent(elClientUrl, engineApiUrl, jwtFilename string) (agent *Agent, err error) {
	var (
		elClient        *client.EthClient
		engineApiClient *client.EngineClient
	)

	if elClient, err = client.NewEthClient(elClientUrl); err != nil {
		return nil, err
	}
	if engineApiClient, err = client.NewEngineClient(engineApiUrl, jwtFilename); err != nil {
		return nil, err
	}

	agent = &Agent{elClient, engineApiClient}
	return agent, nil
}
