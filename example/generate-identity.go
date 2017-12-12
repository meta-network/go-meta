// This file is part of the go-meta library.
//
// Copyright (C) 2017 JAAK MUSIC LTD
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.
//
// If you have any questions please contact yo@jaak.io

package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/meta-network/go-meta/identity"
)

const (
	privKeyHex = "204f8884b5fc4befc878dd82d2e0be2d6f8b93def1fe093cc8724c04287574c8"
)

const usage = `
usage: generate-identity.go USERNAME

Generate a META Identity and print it as JSON.
`

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(1)
	}
	if err := run(os.Args[1]); err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", err)
		os.Exit(1)
	}
}

func run(username string) error {
	key, err := crypto.HexToECDSA(privKeyHex)
	if err != nil {
		return err
	}
	identity := &identity.Identity{
		Username: username,
		Owner:    crypto.PubkeyToAddress(key.PublicKey),
	}
	id := identity.ID()
	signature, err := crypto.Sign(id.Hash[:], key)
	if err != nil {
		return err
	}
	identity.Signature = signature
	data, _ := json.MarshalIndent(identity, "", "  ")
	os.Stdout.Write(data)
	return nil
}
