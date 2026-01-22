package main

import (
	"fmt"
	"os"

	"github.com/linuxdeepin/go-lib/log"
	"github.com/spf13/cobra"
)

var logger = log.NewLogger("lastore/iup-tool")

var updatePlatform UpdatePlatformManager

// initUpdatePlatform initialize update platform manager
func initUpdatePlatform() {
	updatePlatform = UpdatePlatformManager{
		requestURL: getPlatformURLFromDSettings(),
		Token:      getTokenFromAptConfig(),
	}
	// 从 token 中 提取 machine ID
	updatePlatform.machineID = extractMachineIDFromToken(updatePlatform.Token)
}

var rootCmd = &cobra.Command{
	Use:   "iup-tool",
	Short: "tool for intranet update platform operations",
	Long:  "A tool for interacting with the IUP (Intranet Update Platform)",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// set log level
		if globalDebug {
			logger.SetLogLevel(log.LevelDebug)
		} else {
			logger.SetLogLevel(log.LevelInfo)
		}
		logger.Debug("Starting iup-tool")

		// initialize update platform manager
		initUpdatePlatform()
	},
}

var (
	globalTimeout int
	globalDebug   bool
)

func init() {
	// 全局 flags
	rootCmd.PersistentFlags().IntVar(&globalTimeout, "timeout", 40, "HTTP request timeout in seconds")
	rootCmd.PersistentFlags().BoolVar(&globalDebug, "debug", false, "Enable debug logging")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
