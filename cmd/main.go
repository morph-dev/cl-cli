package main

import (
	"os"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/morph-dev/cl-cli/agent"
	"github.com/morph-dev/cl-cli/utils"
	"github.com/urfave/cli/v2"
)

var (
	logLevelFlag = &cli.IntFlag{
		Name:    "loglevel",
		Aliases: []string{"log"},
		Value:   3,
		Usage:   "log level to emit to the screen",
	}
	elClientUrlFlag = &cli.StringFlag{
		Name:  "eth.url",
		Value: "http://127.0.0.1:8545",
		Usage: "Execution layer client endpoint",
	}
	engineApiFlag = &cli.StringFlag{
		Name:  "engine.url",
		Value: "http://127.0.0.1:8551",
		Usage: "Engine api endpoint",
	}
	jwtFilenameFlag = &cli.StringFlag{
		Name:    "engine.jwt",
		Aliases: []string{"jwt"},
		Usage:   "Path to a JWT secret to use for engine API endpoint",
	}

	autoConfirmFlag = &cli.BoolFlag{
		Name:    "auto-confirm",
		Aliases: []string{"auto"},
		Usage:   "Auto confirm all prompts",
	}
	blocksFlag = &cli.IntFlag{
		Name:    "blocks",
		Aliases: []string{"b"},
		Value:   1,
		Usage:   "Number of blocks to create",
	}
	buildBlockCommand = &cli.Command{
		Name:  "build",
		Usage: "Build new block",
		Flags: []cli.Flag{
			autoConfirmFlag,
			blocksFlag,
		},
		Action: buildBlock,
	}
)

func main() {
	app := &cli.App{
		Name:   "cl-cli",
		Usage:  "Cli for emulating Consensus Layer client",
		Before: initApp,
		Flags: []cli.Flag{
			logLevelFlag,
			elClientUrlFlag,
			engineApiFlag,
			jwtFilenameFlag,
		},
		Commands: []*cli.Command{
			buildBlockCommand,
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Error("Application failure", "err", err)
		os.Exit(1)
	}
}

func initApp(ctx *cli.Context) error {
	utils.InitLogger(ctx.Int(logLevelFlag.Name))
	return nil
}

func buildBlock(ctx *cli.Context) (err error) {
	agent, err := agent.NewAgent(
		ctx.String(elClientUrlFlag.Name),
		ctx.String(engineApiFlag.Name),
		ctx.String(jwtFilenameFlag.Name),
	)
	if err != nil {
		return err
	}

	autoConfirm := ctx.Bool(autoConfirmFlag.Name)
	blocks := ctx.Int(blocksFlag.Name)
	for i := 0; i < blocks; i++ {
		if i != 0 {
			time.Sleep(5 * time.Second)
		}

		if err := agent.BuildBlock(autoConfirm); err != nil {
			return err
		}
	}

	return nil
}
