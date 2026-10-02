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

// defaultCmd Represents The Default Command
var defaultCmd = &cobra.Command{
	Use:           "default",
	Short:         "Organize By Default Apple App Categories",
	Args:          cobra.NoArgs,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {

		if Verbose {
			log.SetLevel(log.DebugLevel)
		}

		fmt.Println(command.PorgASCIIArt)

		yesbackup, _ := cmd.Flags().GetBool("backup")
		noBackup, _ := cmd.Flags().GetBool("no-backup")
		yesDefault, _ := cmd.Flags().GetBool("yes")

		backup := false
		if yesbackup {
			backup = true
		} else if noBackup {
			backup = false
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

		if !yesDefault {
			prompt := &survey.Confirm{
				Message: "Organize Launchpad With Default Config?",
			}
			if err := survey.AskOne(prompt, &yesDefault); err == terminal.InterruptErr {
				log.Warn("Exiting...")
				return nil
			}
			if !yesDefault {
				return nil
			}
		}

		log.Info("Apply Default Launchpad Settings")
		return command.DefaultOrg(conf)
	},
}

func init() {
	rootCmd.AddCommand(defaultCmd)

	defaultCmd.Flags().BoolP("yes", "y", false, "Answer Yes To Prompts")
	defaultCmd.Flags().BoolP("backup", "b", false, "Backup Current Launchpad Settings")
	defaultCmd.Flags().BoolP("no-backup", "n", false, "Do Not Backup Current Launchpad Settings")
	defaultCmd.MarkFlagsMutuallyExclusive("backup", "no-backup")
	defaultCmd.SetHelpFunc(func(c *cobra.Command, s []string) {
		rootCmd.PersistentFlags().MarkHidden("config")
		rootCmd.PersistentFlags().MarkHidden("icloud")
		c.Parent().HelpFunc()(c, s)
	})
}
