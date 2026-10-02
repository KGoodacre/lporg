/*
Copyright © 2024 blacktop

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/
package cmd

import (
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/apex/log"
	"github.com/blacktop/lporg/internal/command"
	"github.com/spf13/cobra"
)

// loadCmd Represents The Load Command
var loadCmd = &cobra.Command{
	Use:           "load",
	Short:         "Load Launchpad Settings Config From File",
	Args:          cobra.NoArgs,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {

		if Verbose {
			log.SetLevel(log.DebugLevel)
		}

		fmt.Println(command.PorgASCIIArt)

		yesBackup, _ := cmd.Flags().GetBool("backup")
		noBackup, _ := cmd.Flags().GetBool("no-backup")
		noBackupUpper, _ := cmd.Flags().GetBool("NO")
		yesLoad, _ := cmd.Flags().GetBool("yes")
		yesLoadUpper, _ := cmd.Flags().GetBool("YES")

		// Treat Uppercase And Lowercase Prompt Flags The Same.
		// -y/-Y Means Yes To Prompts.
		// -n/-N Means No To Backup Prompt.
		if noBackupUpper {
			noBackup = true
		}
		if yesLoadUpper {
			yesLoad = true
		}

		backup := false
		if noBackup {
			backup = false
		} else if yesBackup || yesLoad {
			backup = true
		} else {
			prompt := &survey.Confirm{
				Message: "Backup Your Current Launchpad/Dock Settings?",
			}
			if err := survey.AskOne(prompt, &backup); err == terminal.InterruptErr {
				log.Warn("Exiting...")
				return nil
			}
		}

		conf := &command.Config{
			Cmd:      cmd.Use,
			File:     Config,
			Cloud:    UseICloud,
			Backup:   backup,
			LogLevel: setLogLevel(Verbose),
		}

		if err := conf.Verify(); err != nil {
			return err
		}

		if conf.Backup {
			log.Debug("Backing Up Current Launchpad Settings")
			if err := command.SaveConfig(conf); err != nil {
				return err
			}
		}

		if !yesLoad {
			prompt := &survey.Confirm{
				Message: fmt.Sprintf("Load Launchpad Config '%s'?", conf.File),
			}
			if err := survey.AskOne(prompt, &yesLoad); err == terminal.InterruptErr {
				log.Warn("Exiting...")
				return nil
			}
			if !yesLoad {
				return nil
			}
		}

		log.Info("Loading Launchpad Settings")
		return command.LoadConfig(conf)
	},
}

func init() {
	rootCmd.AddCommand(loadCmd)

	loadCmd.Flags().BoolP("backup", "b", false, "Answer Yes To Backup Prompt")
	loadCmd.Flags().BoolP("no-backup", "n", false, "Answer No To Backup Prompt")
	loadCmd.Flags().BoolP("NO", "N", false, "Answer No To Backup Prompt")
	loadCmd.Flags().BoolP("yes", "y", false, "Answer Yes To Prompts")
	loadCmd.Flags().BoolP("YES", "Y", false, "Answer Yes To Prompts")
	loadCmd.MarkFlagsMutuallyExclusive("backup", "no-backup", "NO")
}
