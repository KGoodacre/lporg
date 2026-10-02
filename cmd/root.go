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
	"io"
	"os"
	"sort"
	"strings"

	"github.com/apex/log"
	clihander "github.com/apex/log/handlers/cli"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	// Verbose Boolean Flag For Verbose Logging
	Verbose bool
	// Config Stores The Path To The Config File
	Config string
	// UseICloud Boolean Flag For Using iCloud Config
	UseICloud bool
	// AppVersion Stores The Plugin's Version
	AppVersion string
	// AppBuildTime Stores The Plugin's Build Time
	AppBuildTime string
)

const (
	helpReset  = "\033[0m"
	helpPurple = "\033[1;35m"
	helpCyan   = "\033[1;36m"
	helpGreen  = "\033[1;32m"
	helpYellow = "\033[1;33m"
	helpGray   = "\033[2m"
)

func colorUsage(cmd *cobra.Command) error {
	colorHelp(cmd, nil)
	return nil
}

func colorHelp(cmd *cobra.Command, _ []string) {
	out := cmd.OutOrStdout()

	description := cmd.Long
	if description == "" {
		description = cmd.Short
	}

	fmt.Fprintf(out, "%s%s%s\n", helpPurple, cmd.CommandPath(), helpReset)

	if description != "" {
		fmt.Fprintf(out, "\n%sDESCRIPTION%s\n", helpCyan, helpReset)
		fmt.Fprintf(out, "  %s\n", description)
	}

	fmt.Fprintf(out, "\n%sUSAGE%s\n", helpCyan, helpReset)
	fmt.Fprintf(out, "  %s\n", cmd.UseLine())

	if cmd.HasAvailableSubCommands() {
		commands := []*cobra.Command{}

		for _, child := range cmd.Commands() {
			if child.IsAvailableCommand() || child.Name() == "help" {
				commands = append(commands, child)
			}
		}

		sort.Slice(commands, func(i, j int) bool {
			return commands[i].Name() < commands[j].Name()
		})

		if len(commands) > 0 {
			fmt.Fprintf(out, "\n%sAVAILABLE COMMANDS%s\n", helpCyan, helpReset)

			for _, child := range commands {
				fmt.Fprintf(out, "  %s%-14s%s %s\n", helpGreen, child.Name(), helpReset, child.Short)
			}
		}
	}

	printFlags(out, "FLAGS", cmd.LocalFlags())
	printFlags(out, "GLOBAL FLAGS", cmd.InheritedFlags())

	if cmd.HasExample() {
		fmt.Fprintf(out, "\n%sEXAMPLES%s\n", helpCyan, helpReset)
		fmt.Fprintf(out, "%s\n", cmd.Example)
	}

	if cmd.HasHelpSubCommands() {
		fmt.Fprintf(out, "\n%sADDITIONAL HELP TOPICS%s\n", helpCyan, helpReset)

		for _, child := range cmd.Commands() {
			if child.IsAdditionalHelpTopicCommand() {
				fmt.Fprintf(out, "  %s%-14s%s %s\n", helpGreen, child.CommandPath(), helpReset, child.Short)
			}
		}
	}

	fmt.Fprintf(out, "\n%sUse \"%s --help\" For More Information About A Command.%s\n", helpGray, cmd.CommandPath(), helpReset)
}

func printFlags(out io.Writer, title string, flags *pflag.FlagSet) {
	if flags == nil || !flags.HasAvailableFlags() {
		return
	}

	lines := []string{}

	flags.VisitAll(func(flag *pflag.Flag) {
		if flag.Hidden {
			return
		}

		names := "--" + flag.Name

		if flag.Shorthand != "" {
			names = "-" + flag.Shorthand + ", " + names
		}

		if flag.NoOptDefVal == "" && flag.Value.Type() != "bool" {
			names += " " + strings.ToUpper(flag.Value.Type())
		}

		usage := flag.Usage

		if flag.DefValue != "" && flag.DefValue != "false" {
			usage = fmt.Sprintf("%s %s(Default: %s)%s", usage, helpGray, flag.DefValue, helpReset)
		}

		lines = append(lines, fmt.Sprintf("  %s%-30s%s %s", helpYellow, names, helpReset, usage))
	})

	if len(lines) == 0 {
		return
	}

	sort.Strings(lines)

	fmt.Fprintf(out, "\n%s%s%s\n", helpCyan, title, helpReset)

	for _, line := range lines {
		fmt.Fprintln(out, line)
	}
}

// rootCmd Represents The Base Command When Called Without Any Subcommands
var rootCmd = &cobra.Command{
	Use:   "lporg",
	Short: "Organize Your Launchpad",
	Long:  "Organize, Save, Load, Repair, And Revert macOS Launchpad Layouts.",
}

func setLogLevel(verbose bool) int {
	if verbose {
		return int(log.DebugLevel)
	}
	return int(log.WarnLevel)
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Error(err.Error())
		os.Exit(1)
	}
}

func init() {
	log.SetHandler(clihander.Default)

	rootCmd.SetHelpFunc(colorHelp)
	rootCmd.SetUsageFunc(colorUsage)

	rootCmd.PersistentFlags().BoolVarP(&Verbose, "verbose", "V", false, "Verbose Output")
	rootCmd.PersistentFlags().StringVarP(&Config, "config", "c", "", "Config File (Default Is $CONFIG/lporg/config.yml)")
	rootCmd.PersistentFlags().BoolVar(&UseICloud, "icloud", false, "Use iCloud For Config")
	// Settings
	rootCmd.CompletionOptions.HiddenDefaultCmd = true
}
