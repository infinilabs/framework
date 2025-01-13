// Copyright (C) INFINI Labs & INFINI LIMITED.
//
// The INFINI Framework is offered under the GNU Affero General Public License v3.0
// and as commercial software.
//
// For commercial licensing, contact us at:
//   - Website: infinilabs.com
//   - Email: hello@infini.ltd
//
// Open Source licensed under AGPL V3:
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package keystore

import (
	"bufio"
	"flag"
	"fmt"
	"golang.org/x/crypto/ssh/terminal"
	"infini.sh/framework/core/util"
	kslib "infini.sh/framework/lib/keystore"
	"os"
	"strings"
	"syscall"
)

func RunCmd(args []string) {
	var err error
	keystoreFS := flag.NewFlagSet("manage secrets keystore", flag.ExitOnError)
	keystoreFS.Usage = func() {
		fmt.Printf("usage : keystore <command> [<args>]\n")
		fmt.Printf("These are common keystore commands used in various situations:\n")
		fmt.Printf("add\tAdd keystore secret\n")
		fmt.Printf("list\tList keystore keys\n")
		fmt.Printf("remove\tremove keystore secret\n")
	}
	err = keystoreFS.Parse(args)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	if len(args) == 0 {
		keystoreFS.Usage()
		os.Exit(1)
	}
	cmd, args := args[0], args[1:]
	switch cmd {
	case "add":
		err = addKeystoreValue(args)
	case "list":
		err = listKeystore()
	case "remove":
		err = removeKeystoreSecret(args)
	default:
		fmt.Printf("Unrecognized command %q. "+
			"Command must be one of: add, list, remove\n", cmd)
	}
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	os.Exit(0)
}

func addKeystoreValue(args []string) error {
	addFlag := flag.NewFlagSet("add secret", flag.ExitOnError)
	var (
		stdin = addFlag.Bool("stdin", false, "Use the stdin as the source of the secret")
		force = addFlag.Bool("force", false, "Override the existing key")
	)
	err := addFlag.Parse(args)
	if err != nil {
		return err
	}
	args = addFlag.Args()
	if len(args) == 0 {
		return fmt.Errorf("failed to add the secret: no key provided")
	}
	key := strings.TrimSpace(args[0])
	ks, err := GetOrInitKeystore()
	value, err := ks.Retrieve(key)
	if value != nil && *force == false {
		if *stdin == true {
			return fmt.Errorf("the secret %s already exist in the keystore use `--force` to replace it\n", key)
		}
		answer := util.PromptYesNo(fmt.Sprintf("Secret %s already exists, Overwrite?", key), false)
		if answer == false {
			return fmt.Errorf("Exiting without modifying keystore.")
		}
	}

	var keyValue []byte
	if *stdin {
		reader := bufio.NewReader(os.Stdin)
		keyValue, _, err = reader.ReadLine()
		if err != nil {
			return fmt.Errorf("could not read input from stdin")
		}
	} else {
		fmt.Printf("Enter value for %s: ", key)
		keyValue, err = terminal.ReadPassword(int(syscall.Stdin))
		if err != nil {
			return fmt.Errorf("could not read value from the input, error: %s\n", err)
		}
	}
	ksw, err := kslib.AsWritableKeystore(ks)
	if err != nil {
		return fmt.Errorf("get writeable keystore error: %v\n", err)
	}
	err = ksw.Store(key, keyValue)
	if err != nil {
		return fmt.Errorf("add secret error: %v\n", err)
	}
	err = ksw.Save()
	if err != nil {
		return fmt.Errorf("add secret error: %v\n", err)
	}
	fmt.Println("success")
	return nil
}

func listKeystore() error {
	ks, err := GetOrInitKeystore()
	if err != nil {
		return fmt.Errorf("init keystore error: %v\n", err)
	}
	listKs, err := kslib.AsListingKeystore(ks)
	if err != nil {
		return fmt.Errorf("get list keystore error: %v\n", err)
	}
	keys, err := listKs.List()
	if err != nil {
		return fmt.Errorf("list secrets error: %v\n", err)
	}
	for _, key := range keys {
		fmt.Println(key)
	}
	return nil
}

func removeKeystoreSecret(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("failed to remove the secret: no key provided")
	}
	ks, err := GetOrInitKeystore()
	if err != nil {
		return fmt.Errorf("init keystore error: %v\n", err)
	}
	ksw, err := kslib.AsWritableKeystore(ks)
	if err != nil {
		return fmt.Errorf("get list keystore error: %v\n", err)
	}
	err = ksw.Delete(args[0])
	if err != nil {
		return fmt.Errorf("remove secret error: %v\n", err)
	}
	err = ksw.Save()
	if err != nil {
		return fmt.Errorf("save keystore error: %v\n", err)
	}
	return nil
}
