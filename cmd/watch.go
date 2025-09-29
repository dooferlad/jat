/*
Copyright © 2020 James Tunnicliffe <dooferlad@nanosheep.org>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"fmt"
	"regexp"
	"time"

	"github.com/dooferlad/jat/shell"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var match string
var mismatch string

// watchCmd represents the watch command
var watchCmd = &cobra.Command{
	Use:   "watch [flags] <command> -- [command flags]",
	Short: "watch a command, log output",
	Long: `Watch the given command, log the output

If match or mismatch are provided only log lines that match/don't match those
flags.

To pass arguments to the command you are watching, prefix the list with "--", e.g.

  # watch the command "ls -R -X"
  jat watch -m go ls -- -R -X

`,

	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("no command given to watch")
		}

		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		m, err := regexp.Compile(match)
		if err != nil {
			return err
		}

		v, err := regexp.Compile(mismatch)
		if err != nil {
			return err
		}

		command := args[0]
		var commandArgs []string
		if len(args) > 1 {
			commandArgs = args[1:len(args)]
		}

		for {
			start := time.Now()
			out, err := shell.Capture(command, commandArgs...)
			if err != nil {
				return err
			}

			if match != "" && m.Match(out) {
				logrus.Info(string(out))
			} else if mismatch != "" && !v.Match(out) {
				logrus.Info(string(out))
			} else if match == "" && mismatch == "" {
				logrus.Info(string(out))
			}

			duration := time.Since(start)
			if duration < time.Second {
				time.Sleep(time.Second - duration)
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(watchCmd)

	watchCmd.Flags().StringVarP(&match, "match", "m", "", "log matching output")
	watchCmd.Flags().StringVarP(&mismatch, "mismatch", "v", "", "log non-matching output")

	logrus.SetFormatter(&logrus.TextFormatter{
		// DisableColors: true,
		FullTimestamp: true,
	})
}
