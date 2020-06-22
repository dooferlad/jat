// Copyright © 2018 James Tunnicliffe <jat@nanosheep.org>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"fmt"
	"github.com/dooferlad/jat/shell"

	"github.com/spf13/cobra"
)

// shutdownCmd represents the shutdown command
var shutdownCmd = &cobra.Command{
	Use:   "shutdown",
	Short: "trim file systems, update, power off",
	RunE: func(cmd *cobra.Command, args []string) error {
		return Shutdown()
	},
}

// Shutdown updates the machine, tidies up, and shuts down
func Shutdown() error {
	if err := Upgrade(); err != nil {
		return err
	}

	if err := shell.Sudo("fstrim", "--all"); err != nil {
		return fmt.Errorf("trimming file systems: %s", err)
	}

	if err := shell.Sudo("halt", "-p"); err != nil {
		return fmt.Errorf("updating packages: %s", err)
	}

	return nil
}

func init() {
	RootCmd.AddCommand(shutdownCmd)
}
