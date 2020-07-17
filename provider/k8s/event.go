package k8s

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"

	"github.com/convox/convox/pkg/common"
	"github.com/convox/convox/pkg/structs"
)

type event struct {
	Action    string            `json:"action"`
	Data      map[string]string `json:"data"`
	Status    string            `json:"status"`
	Timestamp time.Time         `json:"timestamp"`
}

func (p *Provider) EventSend(action string, opts structs.EventSendOptions) error {
	e := event{
		Action:    action,
		Data:      opts.Data,
		Status:    common.DefaultString(opts.Status, "success"),
		Timestamp: time.Now().UTC(),
	}

	if e.Data["timestamp"] != "" {
		t, err := time.Parse(time.RFC3339, e.Data["timestamp"])
		if err == nil {
			e.Timestamp = t
		}
	}

	if opts.Error != nil {
		e.Status = "error"
		e.Data["message"] = *opts.Error
	}

	e.Data["rack"] = p.Name

	msg, err := json.Marshal(e)
	if err != nil {
		return err
	}

	for _, wh := range p.webhooks {
		go dispatchWebhook(wh, msg)
	}

	return nil
}

func dispatchWebhook(url string, body []byte) error {
	res, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer res.Body.Close()

	return nil
}
