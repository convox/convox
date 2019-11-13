package aws

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/convox/convox/pkg/common"
	"github.com/convox/convox/pkg/structs"
)

var sequenceTokens sync.Map

func (p *Provider) Log(app, stream string, ts time.Time, message string) error {
	group := p.appLogGroup(app)

	req := &cloudwatchlogs.PutLogEventsInput{
		LogGroupName:  aws.String(group),
		LogStreamName: aws.String(stream),
		LogEvents: []*cloudwatchlogs.InputLogEvent{
			{
				Timestamp: aws.Int64(ts.UnixNano() / int64(time.Millisecond)),
				Message:   aws.String(message),
			},
		},
	}

	key := fmt.Sprintf("%s/%s", *req.LogGroupName, *req.LogStreamName)

	if tv, ok := sequenceTokens.Load(key); ok {
		if token, ok := tv.(string); ok {
			req.SequenceToken = aws.String(token)
		}
	}

	for {
		res, err := p.CloudWatchLogs.PutLogEvents(req)
		switch awsErrorCode(err) {
		case "ResourceNotFoundException":
			if strings.Contains(err.Error(), "log group") {
				if err := p.createLogGroup(app); err != nil {
					return err
				}
			}
			if err := p.createLogStream(group, stream); err != nil {
				return err
			}
		case "InvalidSequenceTokenException":
			token, err := p.nextSequenceToken(group, stream)
			if err != nil {
				return err
			}
			req.SequenceToken = aws.String(token)
		case "":
			sequenceTokens.Store(key, *res.NextSequenceToken)
			return nil
		default:
			return err
		}

		continue
	}

	return nil
}

func (p *Provider) AppLogs(name string, opts structs.LogsOptions) (io.ReadCloser, error) {
	return p.subscribeLogs(p.Context(), p.appLogGroup(name), "", opts)
}

func (p *Provider) SystemLogs(opts structs.LogsOptions) (io.ReadCloser, error) {
	return p.subscribeLogs(p.Context(), p.appLogGroup("system"), "", opts)
}

func (p *Provider) appLogGroup(app string) string {
	return fmt.Sprintf("/convox/%s/%s", p.Name, app)
}

func (p *Provider) createLogGroup(app string) error {
	_, err := p.CloudWatchLogs.CreateLogGroup(&cloudwatchlogs.CreateLogGroupInput{
		LogGroupName: aws.String(p.appLogGroup(app)),
		Tags: map[string]*string{
			"system": aws.String("convox"),
			"rack":   aws.String(p.Name),
			"app":    aws.String(app),
		},
	})
	if err != nil {
		return err
	}

	return nil
}

func (p *Provider) createLogStream(group, stream string) error {
	_, err := p.CloudWatchLogs.CreateLogStream(&cloudwatchlogs.CreateLogStreamInput{
		LogGroupName:  aws.String(group),
		LogStreamName: aws.String(stream),
	})
	if err != nil {
		return err
	}

	return nil
}

func (p *Provider) nextSequenceToken(group, stream string) (string, error) {
	res, err := p.CloudWatchLogs.DescribeLogStreams(&cloudwatchlogs.DescribeLogStreamsInput{
		LogGroupName:        aws.String(group),
		LogStreamNamePrefix: aws.String(stream),
	})
	if err != nil {
		return "", err
	}
	if len(res.LogStreams) != 1 {
		return "", fmt.Errorf("could not describe log stream: %s/%s", group, stream)
	}
	if res.LogStreams[0].UploadSequenceToken == nil {
		return "", fmt.Errorf("could not fetch sequence token for log stream: %s/%s", group, stream)
	}

	return *res.LogStreams[0].UploadSequenceToken, nil
}

func (p *Provider) streamLogs(ctx context.Context, w io.WriteCloser, group, stream string, opts structs.LogsOptions) error {
	defer w.Close()

	req := &cloudwatchlogs.FilterLogEventsInput{
		LogGroupName: aws.String(group),
	}

	if opts.Filter != nil {
		req.FilterPattern = aws.String(*opts.Filter)
	}

	follow := common.DefaultBool(opts.Follow, true)

	var start int64

	if opts.Since != nil {
		start = time.Now().UTC().Add((*opts.Since)*-1).UnixNano() / int64(time.Millisecond)
		req.StartTime = aws.Int64(start)
	}

	if stream != "" {
		req.LogStreamNames = []*string{aws.String(stream)}
	} else {
		req.Interleaved = aws.Bool(true)
	}

	seen := map[string]bool{}

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			// check for closed writer
			if _, err := w.Write([]byte{}); err != nil {
				return err
			}

			res, err := p.CloudWatchLogs.FilterLogEvents(req)
			if err != nil {
				switch awsErrorCode(err) {
				case "ThrottlingException", "ResourceNotFoundException":
					time.Sleep(1 * time.Second)
					continue
				default:
					return err
				}
			}

			es := []*cloudwatchlogs.FilteredLogEvent{}

			for _, e := range res.Events {
				if !seen[*e.EventId] {
					es = append(es, e)
					seen[*e.EventId] = true
				}

				if e.Timestamp != nil && *e.Timestamp > start {
					start = *e.Timestamp
				}
			}

			sort.Slice(es, func(i, j int) bool { return *es[i].Timestamp < *es[j].Timestamp })

			if _, err := writeLogEvents(w, es, opts); err != nil {
				return err
			}

			req.NextToken = res.NextToken

			if res.NextToken == nil {
				if !follow {
					return nil
				}

				req.StartTime = aws.Int64(start)
			}
		}
	}
}

func (p *Provider) subscribeLogs(ctx context.Context, group, stream string, opts structs.LogsOptions) (io.ReadCloser, error) {
	r, w := io.Pipe()

	go p.streamLogs(ctx, w, group, stream, opts)

	return r, nil
}

func writeLogEvents(w io.Writer, events []*cloudwatchlogs.FilteredLogEvent, opts structs.LogsOptions) (int64, error) {
	if len(events) == 0 {
		return 0, nil
	}

	latest := int64(0)

	for _, e := range events {
		if *e.Timestamp > latest {
			latest = *e.Timestamp
		}

		prefix := ""

		if common.DefaultBool(opts.Prefix, false) {
			sec := *e.Timestamp / 1000
			nsec := (*e.Timestamp % 1000) * 1000
			t := time.Unix(sec, nsec).UTC()

			prefix = fmt.Sprintf("%s %s ", t.Format(time.RFC3339), *e.LogStreamName)
		}

		line := fmt.Sprintf("%s%s\n", prefix, strings.TrimSuffix(*e.Message, "\n"))

		if _, err := w.Write([]byte(line)); err != nil {
			return 0, err
		}
	}

	return latest, nil
}
