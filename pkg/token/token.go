// +build !windows

package token

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ddollar/go-u2fhost"
)

type authenticationKey struct {
	AppId     string `json:"appId"`
	KeyHandle string `json:"keyHandle"`
	Version   string `json:"version"`
}

type authenticationRequest struct {
	AppId          string `json:"appId"`
	Challenge      string `json:"challenge"`
	RegisteredKeys []authenticationKey
}

type authenticationResponse struct {
	ClientData    string `json:"clientData"`
	KeyHandle     string `json:"keyHandle"`
	SignatureData string `json:"signatureData"`
}

type tokenResponse struct {
	Error    error
	Response *u2fhost.AuthenticateResponse
}

func Authenticate(req []byte) ([]byte, error) {
	ds := u2fhost.Devices()

	ch := make(chan tokenResponse)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var areq authenticationRequest

	if err := json.Unmarshal(req, &areq); err != nil {
		return nil, err
	}

	for _, d := range ds {
		go authenticateDevice(ctx, d, areq, ch)
	}

	for range ds {
		res := <-ch

		if res.Error != nil {
			return nil, res.Error
		}

		if res.Response != nil {
			ares := authenticationResponse{
				ClientData:    res.Response.ClientData,
				KeyHandle:     res.Response.KeyHandle,
				SignatureData: res.Response.SignatureData,
			}

			data, err := json.Marshal(ares)
			if err != nil {
				return nil, err
			}

			return data, nil
		}
	}

	return nil, fmt.Errorf("no valid tokens found")
}

func authenticateDevice(ctx context.Context, d *u2fhost.HidDevice, req authenticationRequest, rch chan tokenResponse) {
	if err := d.Open(); err != nil {
		rch <- tokenResponse{Error: err}
		return
	}

	defer d.Close()

	tick := time.NewTicker(250 * time.Millisecond)
	defer tick.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			for _, k := range req.RegisteredKeys {
				areq := &u2fhost.AuthenticateRequest{
					AppId:     k.AppId,
					Challenge: req.Challenge,
					Facet:     req.AppId,
					KeyHandle: k.KeyHandle,
				}

				ares, err := d.Authenticate(areq)
				switch err.(type) {
				case *u2fhost.BadKeyHandleError:
				case *u2fhost.TestOfUserPresenceRequiredError:
				case nil:
					rch <- tokenResponse{Response: ares}
					return
				default:
					rch <- tokenResponse{Error: err}
					return
				}
			}
		}
	}
}
