package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/Bai-Tian/minecraft-mod-synchronizer/cmd"
	"github.com/spf13/cobra"
)

func main() {
	defaultSettings, raw, valid_len, err := cmd.ReadClientSettingsWithRaw()
	if err != nil {
		log.Fatalf("internal error: can not read default settings: %v", err)
	}

	var input string
	var output string
	var settings cmd.ClientSettings

	rootCmd := &cobra.Command{
		Use:   "tool INPUT",
		Short: "Tool for modify constants in client binary",
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) < 1 {
				fmt.Println("input file is required")
				cmd.Help()
			}
			input = args[0]
			if output == "" {
				ext_ind := strings.LastIndexByte(input, '.')
				if ext_ind > 0 {
					output = input[:ext_ind] + "-new" + input[ext_ind:]
				} else {
					output = input + "-new"
				}
			}
			log.Printf("Settings: %v\n", settings)
			log.Printf("Writing settings to %v\n", output)
			log.Printf("Raw: %d/%d\n", valid_len, len(raw))

			settings_data, err := json.Marshal(settings)
			if err != nil {
				log.Fatalf("internal error: can not marshal settings: %v", err)
			}
			if len(settings_data) > len(raw) {
				log.Fatalf("new settings is larger than old settings")
			}

			bin, err := os.ReadFile(input)
			if err != nil {
				log.Fatalf("can not read input file: %v", err)
			}
			constant_index := bytes.Index(bin, raw)
			if constant_index < 0 {
				log.Fatalf("can not find constant of settings in input file")
			}
			constant_index += copy(bin[constant_index:], settings_data)
			if len(settings_data) < valid_len {
				tmp := bytes.Repeat([]byte{'X'}, valid_len-len(settings_data))
				copy(bin[constant_index:], tmp)
			}
			err = os.WriteFile(output, bin, 0666)
			if err != nil {
				log.Fatalf("can not write output file: %v", err)
			}
		},
	}

	rootCmd.PersistentFlags().StringVarP(&output, "output", "o", "", "Output binary file")
	rootCmd.PersistentFlags().StringVar(&settings.ServerAddress, "server-address", defaultSettings.ServerAddress, "ClientSettings::ServerAddress")
	rootCmd.PersistentFlags().StringVar(&settings.ListEndpoint, "list-endpoint", defaultSettings.ListEndpoint, "ClientSettings::ListEndpoint")
	rootCmd.PersistentFlags().StringVar(&settings.DownloadEndpoint, "download-endpoint", defaultSettings.DownloadEndpoint, "ClientSettings::DownloadEndpoint")
	rootCmd.PersistentFlags().StringVar(&settings.DownloadPrompt, "download-prompt", defaultSettings.DownloadPrompt, "ClientSettings::DownloadPrompt")
	rootCmd.PersistentFlags().StringVar(&settings.ConfigFile, "config-file", defaultSettings.ConfigFile, "ClientSettings::ConfigFile")

	err = rootCmd.Execute()
	if err != nil {
		log.Fatalf("invalid args: %v", err)
	}

}
