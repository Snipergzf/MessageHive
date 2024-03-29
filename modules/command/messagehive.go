package command

import (
	"strings"

	"github.com/Snipergzf/MessageHive/modules/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var MessageHiveCmd = &cobra.Command{
	Use:   "MessageHive",
	Short: "MessageHive is a expressive, fast, full featured message gate.",
	Long:  "A expressivee, fast, full featured message gate lovely built by Hongcai Deng",
	Run: func(cmd *cobra.Command, args []string) {
		InitializeConfig()
	},
}

var messagehiveCmdV *cobra.Command

var CfgFile, LogLevel string

func Execute() {
	AddCommands()
	MessageHiveCmd.Execute()
}

func AddCommands() {
	MessageHiveCmd.AddCommand(serverCmd)
}

func init() {
	MessageHiveCmd.PersistentFlags().StringVarP(&CfgFile, "config", "c", "config.json", "config file (default is path/config.json)")
	MessageHiveCmd.PersistentFlags().StringVar(&LogLevel, "logLevel", "Info", "logout put level")
	messagehiveCmdV = MessageHiveCmd
}

func InitializeConfig() {
	viper.SetConfigFile(CfgFile)
	viper.SetConfigType("json")
	err := viper.ReadInConfig()
	if err != nil {
		panic("Unable to locate Config file.Perhaps you need to create a config from example")
	}
	viper.SetDefault("log", map[string]string{"level": "Info"})

	if messagehiveCmdV.PersistentFlags().Lookup("logLevel").Changed {
		viper.Set("log.level", LogLevel)
	}

	log.NewLogger("console", `{"level": "`+strings.Title(viper.GetString("log.level"))+`"}`)

	log.Info("Using config file: %s", viper.ConfigFileUsed())
}
