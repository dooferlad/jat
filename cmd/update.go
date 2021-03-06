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

	"github.com/dooferlad/jat/dpkg"
	"github.com/dooferlad/jat/shell"
	"github.com/spf13/cobra"
)

func Upgrade() error {
	if err := dpkg.Update(); err != nil {
		return err
	}

	if err := shell.Sudo("apt", "update"); err != nil {
		return fmt.Errorf("updating packages: %s", err)
	}

	if err := shell.Sudo("apt", "upgrade", "-y", "--autoremove"); err != nil {
		return fmt.Errorf("updating packages: %s", err)
	}

	return nil
}

// updateCmd represents the update command
var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update software on this machine",
	RunE: func(cmd *cobra.Command, args []string) error {
		return Upgrade()
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
}
