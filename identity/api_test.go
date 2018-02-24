// This file is part of the go-meta library.
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

package identity

import (
	"bytes"
	"net/http/httptest"
	"testing"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/meta-network/go-meta/graph"
	"github.com/meta-network/go-meta/testutil"
)

func TestIdentityAPI(t *testing.T) {
	// create a test API
	dpa, err := testutil.NewTestDPA()
	if err != nil {
		t.Fatal(err)
	}
	defer dpa.Cleanup()
	registry := testutil.NewTestRegistry()
	driver := graph.NewDriver("meta-id-test", dpa.DPA, registry, dpa.Dir)
	api, err := NewAPI(driver)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(api)
	defer srv.Close()

	// create a graph
	client := NewClient(srv.URL + "/graphql")
	hash, err := client.CreateGraph(testMetaID.Hex())
	if err != nil {
		t.Fatal(err)
	}
	sig, err := crypto.Sign(hash[:], testKey)
	if err != nil {
		t.Fatal(err)
	}
	if err := registry.SetGraph(hash, sig); err != nil {
		t.Fatal(err)
	}

	// create a claim
	claim := newTestClaim(t, "username", "test")
	hash, err = client.CreateClaim(testMetaID.Hex(), claim)
	if err != nil {
		t.Fatal(err)
	}
	sig, err = crypto.Sign(hash[:], testKey)
	if err != nil {
		t.Fatal(err)
	}
	if err := registry.SetGraph(hash, sig); err != nil {
		t.Fatal(err)
	}

	// get the claim
	id := testMetaID.Hex()
	claims, err := client.Claim(testMetaID.Hex(), &ClaimFilter{
		Issuer:   &id,
		Subject:  &id,
		Property: &claim.Property,
		Claim:    &claim.Claim,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(claims) != 1 {
		t.Fatalf("expected 1 claim, got %d", len(claims))
	}
	gotClaim := claims[0]
	if gotClaim.ID() != claim.ID() {
		t.Fatalf("expected claim to have ID %s, got %s", claim.ID().String(), gotClaim.ID().String())
	}
	if !bytes.Equal(gotClaim.Signature, claim.Signature) {
		t.Fatalf("expected claim to have signature %s, got %s", hexutil.Encode(claim.Signature), hexutil.Encode(gotClaim.Signature))
	}
}

var (
	testKey, _ = crypto.HexToECDSA("289c2857d4598e37fb9647507e47a309d6133539bf21a8b9cb6df88fd5232032")
	testMetaID = NewID(crypto.PubkeyToAddress(testKey.PublicKey))
)

func newTestClaim(t *testing.T, property, claim string) *Claim {
	c := &Claim{
		Issuer:   testMetaID,
		Subject:  testMetaID,
		Property: property,
		Claim:    claim,
	}
	id := c.ID()
	signature, err := crypto.Sign(id[:], testKey)
	if err != nil {
		t.Fatal(err)
	}
	c.Signature = signature
	return c
}
