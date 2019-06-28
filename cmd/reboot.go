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

	"github.com/spf13/cobra"
)

func Reboot() error {
	if err := Sudo("reboot"); err != nil {
		return fmt.Errorf("rebooting: %s", err)
	}

	return nil
}

// rebootCmd represents the reboot command
var rebootCmd = &cobra.Command{
	Use:   "reboot",
	Short: "Update software and reboot",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := Upgrade(); err != nil {
			return err
		}

		if err := Reboot(); err != nil {
			return err
		}

		return nil
	},
}

func init() {
	RootCmd.AddCommand(rebootCmd)
}
