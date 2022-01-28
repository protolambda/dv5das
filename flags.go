package main

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	gcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/enr"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/libp2p/go-libp2p-core/crypto"
	"strings"
)

type P2pPrivKeyFlag crypto.Secp256k1PrivateKey

func (f *P2pPrivKeyFlag) String() string {
	if f == nil {
		return "? (no private key data)"
	}
	secpKey := (*crypto.Secp256k1PrivateKey)(f)
	keyBytes, err := secpKey.Raw()
	if err != nil {
		return "? (invalid private key)"
	}
	return hex.EncodeToString(keyBytes)
}

func (f *P2pPrivKeyFlag) Set(value string) error {
	// No private key if no data
	if value == "" {
		return fmt.Errorf("no priv key specified")
	}
	var priv *crypto.Secp256k1PrivateKey
	var err error
	priv, err = ParsePrivateKey(value)
	if err != nil {
		return fmt.Errorf("could not parse private key: %v", err)
	}
	*(*crypto.Secp256k1PrivateKey)(f) = *priv
	return nil
}

func (f *P2pPrivKeyFlag) Type() string {
	return "P2P Private key"
}

func ParsePrivateKey(v string) (*crypto.Secp256k1PrivateKey, error) {
	if strings.HasPrefix(v, "0x") {
		v = v[2:]
	}
	privKeyBytes, err := hex.DecodeString(v)
	if err != nil {
		return nil, fmt.Errorf("cannot parse private key, expected hex string: %v", err)
	}
	var priv crypto.PrivKey
	priv, err = crypto.UnmarshalSecp256k1PrivateKey(privKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("cannot parse private key, invalid private key (Secp256k1): %v", err)
	}
	key := (priv).(*crypto.Secp256k1PrivateKey)
	key.Curve = gcrypto.S256()              // Temporary hack, so libp2p Secp256k1 is recognized as geth Secp256k1 in disc v5.1
	if !key.Curve.IsOnCurve(key.X, key.Y) { // TODO: should we be checking this?
		return nil, fmt.Errorf("invalid private key, not on curve")
	}
	return key, nil
}

func ParseEnode(v string) (*enode.Node, error) {
	addr := new(enode.Node)
	err := addr.UnmarshalText([]byte(v))
	if err != nil {
		return nil, err
	}
	return addr, nil
}

func ParseEnrBytes(v string) ([]byte, error) {
	if strings.HasPrefix(v, "enr:") {
		v = v[4:]
		if strings.HasPrefix(v, "//") {
			v = v[2:]
		}
	}
	return base64.RawURLEncoding.DecodeString(v)
}

func ParseEnr(v string) (*enr.Record, error) {
	data, err := ParseEnrBytes(v)
	if err != nil {
		return nil, err
	}
	var record enr.Record
	if err := rlp.Decode(bytes.NewReader(data), &record); err != nil {
		return nil, err
	}
	return &record, nil
}

func EnrToEnode(record *enr.Record, verifySig bool) (*enode.Node, error) {
	idSchemeName := record.IdentityScheme()

	if verifySig {
		if err := record.VerifySignature(enode.ValidSchemes[idSchemeName]); err != nil {
			return nil, err
		}
	}

	return enode.New(enode.ValidSchemes[idSchemeName], record)
}

func ParseEnrOrEnode(v string) (*enode.Node, error) {
	if strings.HasPrefix(v, "enode://") {
		return ParseEnode(v)
	} else {
		enrAddr, err := ParseEnr(v)
		if err != nil {
			return nil, err
		}
		enodeAddr, err := EnrToEnode(enrAddr, true)
		if err != nil {
			return nil, err
		}
		return enodeAddr, nil
	}
}
