package sdk

import (
	"bytes"
	"encoding/json"
	"io/ioutil"

	"github.com/convox/convox/pkg/structs"
	"github.com/convox/stdsdk"
)

func (c *Client) Auth() (string, error) {
	res, err := c.GetStream("/auth", stdsdk.RequestOptions{})
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	var auth struct {
		Id string
	}

	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", err
	}

	if err := json.Unmarshal(data, &auth); err == nil {
		return auth.Id, nil
	}

	return "", nil
}

func (c *Client) SsoAuth(opts structs.SsoAuthOptions) (string, error) {
	ro := stdsdk.RequestOptions{Headers: stdsdk.Headers{}, Params: stdsdk.Params{}, Query: stdsdk.Query{}}
	body, _ := json.Marshal(opts)
	ro.Body = bytes.NewReader(body)

	res, err := c.PostStream("/sso/auth", ro)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	var auth struct {
		Id string
	}

	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", err
	}

	if err := json.Unmarshal(data, &auth); err == nil {
		return auth.Id, nil
	}

	return "", nil
}
