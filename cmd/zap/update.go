package main

import (
	"fmt"
	"os"

	"github.com/blang/semver"
	"github.com/rhysd/go-github-selfupdate/selfupdate"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(updateCmd)
}

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update ZAP to the latest version",
	Run: func(cmd *cobra.Command, args []string) {
		if version == "dev" {
			fmt.Println("You are running a development version of ZAP. Update is not supported.")
			return
		}

		latest, found, err := selfupdate.DetectLatest("blackcoderx/zap")
		if err != nil {
			fmt.Println("Error occurred while detecting version:", err)
			return
		}

		v, err := semver.Parse(version)
		if err != nil {
			fmt.Printf("Error parsing current version '%s': %v\n", version, err)
			return
		}

		if !found || latest.Version.LTE(v) {
			fmt.Println("Current version is the latest")
			return
		}

		fmt.Print("Do you want to update to ", latest.Version, "? (y/n): ")
		var input string
		fmt.Scanln(&input)
		if input != "y" {
			return
		}

		exe, err := os.Executable()
		if err != nil {
			fmt.Println("Could not locate executable path")
			return
		}
		if err := selfupdate.UpdateTo(latest.AssetURL, exe); err != nil {
			fmt.Println("Error occurred while updating binary:", err)
			return
		}
		fmt.Println("Successfully updated to version", latest.Version)
	},
}
