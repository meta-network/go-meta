// This file is part of the go-kord library.
//
// Copyright (C) 2018 JAAK MUSIC LTD
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

package dapp

import (
	"github.com/cayleygraph/cayley/quad"
	"github.com/cayleygraph/cayley/voc"
)

func init() {
	voc.RegisterPrefix("dapp:", "http://schema.kord-network.io/dapp/")
}

type Dapp struct {
	ID           quad.IRI `quad:"@id"`
	ManifestHash string   `quad:"dapp:manifestHash"`
}
